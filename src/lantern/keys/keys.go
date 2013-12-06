/*
Package keys encapsulates the key and certificate management for this lantern
instance, including generating them, saving them to disk, using them to
encrypt/decrypt data and using them to trust peers via TLS connections.

Package keys also includes functionality to handle remote certificate generation
whereby parent nodes generate certificates for their children, whom they
initially authenticate using Mozilla Persona.

Keys and certificates are stored in [config.ConfigDir]/keys, with the following
directory structure:

own/
    privatekey.pem (our private key)
	certificate.pem (our certificate)
trusted/
	parentcert.pem (our parent's certificate)

Any and all of these can be prepopulated with pregenerated values, which keys
will happily use.  For child nodes, parentcert.pem has to be prepopulated,
meaning that that part of the key exchange has to happen out of band (for
example via email).  privatekey.pem and certificate.pem will be generated
as necessary.

TODO: handle certificate expirations to make sure we rotate certificates
frequently.
*/
package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"lantern/config"
	"log"
	"math/big"
	"net"
	"os"
	"reflect"
	"sync"
	"time"
)

const (
	PEM_HEADER_PRIVATE_KEY = "RSA PRIVATE KEY"
	PEM_HEADER_PUBLIC_KEY  = "RSA PRIVATE KEY"
	PEM_HEADER_CERTIFICATE = "CERTIFICATE"
	KEY_BITS               = 2048
	ONE_WEEK               = 7 * 24 * time.Hour
	TWO_WEEKS              = ONE_WEEK * 2
)

var (
	PrivateKeyFile  string               // the location of our private key on disk
	CertificateFile string               // the location of our certificate on disk
	TrustedParents  = x509.NewCertPool() // pool of trusted parent certificates
)

func PrivateKey() *rsa.PrivateKey {
	return privateKey
}

// Certificate() returns our certificate and, if there's no certificate,
// a channel from which the certificate can be obtained.
func Certificate() (*x509.Certificate, chan *x509.Certificate) {
	certMutex.RLock()
	defer certMutex.RUnlock()
	if certificate != nil {
		return certificate, nil
	} else {
		certMutex.Lock()
		defer certMutex.Unlock()
		waitingForCert := make(chan *x509.Certificate)
		waitingForCerts = append(waitingForCerts, waitingForCert)
		return nil, waitingForCert
	}
}

// Encrypt() encrypts the given string and returns it as a base64 encoded string
func Encrypt(value string) (string, error) {
	if bytes, err := rsa.EncryptPKCS1v15(rand.Reader, &(privateKey.PublicKey), []byte(value)); err != nil {
		return "", err
	} else {
		return base64.StdEncoding.EncodeToString(bytes), nil
	}
}

// Decrypt() decryptes a string value from the given base64 encoded string
func Decrypt(value string) (string, error) {
	if bytes, err := base64.StdEncoding.DecodeString(value); err != nil {
		return "", err
	} else {
		if bytes, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, bytes); err != nil {
			return "", err
		} else {
			return string(bytes), nil
		}
	}
}

var (
	privateKey      *rsa.PrivateKey                     // our private key
	certificate     *x509.Certificate                   // our certificate
	parentCertFile  string                              // our parent's certificate
	certMutex       sync.RWMutex                        // used to synchronize access to our certificate
	waitingForCerts = make([]chan *x509.Certificate, 0) // callbacks of parties waiting for us to get/generate a cert
)

func init() {
	log.Print("Configuring keys")
	ownPath := config.ConfigDir + "/keys/own/"
	trustedPath := config.ConfigDir + "/keys/trusted/"
	PrivateKeyFile = ownPath + "privatekey.pem"
	CertificateFile = ownPath + "certificate.pem"
	parentCertFile = trustedPath + "parentcert.pem"
	if err := os.MkdirAll(ownPath, 0755); err != nil {
		log.Fatalf("Unable to create directory for own keys '%s': %s", ownPath, err)
	}
	if !config.IsRootNode() {
		loadParentCert()
	}
	loadPrivateKey()
	loadCertificate()
}

// loadPrivateKey() loads our private key from disk and, if not found, creates it
func loadPrivateKey() {
	if privateKeyData, err := ioutil.ReadFile(PrivateKeyFile); err != nil {
		log.Print("Unable to read private key file from disk, creating")
		createPrivateKey()
	} else {
		block, _ := pem.Decode(privateKeyData)
		if block == nil {
			log.Print("Unable to decode PEM encoded private key data, creating")
			createPrivateKey()
		} else {
			privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				log.Print("Unable to decode X509 private key data, creating")
				createPrivateKey()
			} else {
				log.Printf("Read private key")
			}
		}
	}
}

// createPrivateKey() creates an RSA private key and saves it to disk
func createPrivateKey() {
	newPrivateKey, err := rsa.GenerateKey(rand.Reader, KEY_BITS)
	if err != nil {
		log.Fatalf("Failed to generate private key: %s", err)
	}

	privateKey = newPrivateKey
	keyOut, err := os.OpenFile(PrivateKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Failed to open %s for writing: %s", PrivateKeyFile, err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: PEM_HEADER_PRIVATE_KEY, Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}); err != nil {
		log.Fatalf("Unable to PEM encode private key: %s", err)
	}
	keyOut.Close()
	log.Printf("Wrote private key to %s", PrivateKeyFile)
}

// loadParentCert() loads the parent cert from disk
func loadParentCert() {
	if certificateData, err := ioutil.ReadFile(parentCertFile); err != nil {
		log.Fatal("Unable to read parent certificate file from disk")
	} else {
		if TrustedParents.AppendCertsFromPEM(certificateData) {
			log.Print("Added trusted parent cert")
		} else {
			log.Fatal("Unable to add trusted parent cert")
		}
	}
}

/*
loadCertificate() loads our certificate from disk, or if it doesn't exist,
initialize it either by requesting a cert from our parent (if we have one) or
generating a self-signed certificate (if we're a root node).
*/
func loadCertificate() {
	certMutex.Lock()
	defer certMutex.Unlock()
	if certificateData, err := ioutil.ReadFile(CertificateFile); err != nil {
		log.Printf("Unable to read certificate file from disk: %s", err)
		initCertificate()
	} else {
		block, _ := pem.Decode(certificateData)
		if block == nil {
			log.Print("Unable to decode PEM encoded certificate")
			initCertificate()
		} else {
			certificate, err = x509.ParseCertificate(block.Bytes)
			if err != nil {
				log.Print("Unable to decode X509 certificate data")
				initCertificate()
			}
			log.Printf("Read certificate")
		}
	}

	// Add ourselves to the trust store
	TrustedParents.AddCert(certificate)
}

/*
initCertificate() initializes our certificate either by requesting a cert from
our parent (if we have one) or generating a self-signed certificate (if we're a
root node).
*/
func initCertificate() {
	var derBytes []byte
	var err error
	if config.IsRootNode() {
		log.Print("This is a root node, generating self-signed certificate")
		derBytes, err = certificateForPublicKey("", &privateKey.PublicKey)
		if err != nil {
			log.Fatalf("Unable to generate self-signed certificate: %s", err)
		}
	} else {
		log.Print("We have a parent, requesting a certificate from parent")
		publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		if err != nil {
			log.Fatalf("Unable to get DER encoded bytes for public key: %s", err)
		}
		derBytes, err = requestCertFromParent(publicKeyBytes)
		if err != nil {
			log.Fatalf("Unable to request certificate from parent: %s", err)
		}
	}

	saveCertificate(derBytes)

	// Notify anyone waiting for a cert
	for _, waitingForCert := range waitingForCerts {
		waitingForCert <- certificate
	}
}

/*
Same as certificateForPublicKey(), with the public key supplied as the DER bytes.
*/
func certificateForBytes(email string, publicKeyBytes []byte) ([]byte, error) {
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyBytes)
	if err != nil {
		return nil, err
	}
	switch pk := publicKey.(type) {
	case *rsa.PublicKey:
		certificateBytes, err := certificateForPublicKey(email, pk)
		if err != nil {
			return nil, err
		}
		return certificateBytes, nil
	default:
		return nil, fmt.Errorf("Unsupported key type: %s", reflect.TypeOf(pk))
	}
}

/*
certificateForPublicKey() creates a certificate from the given public key,
returning DER bytes for the Certificate.  The supplied email is encrypted and
stored as the common name so that the issuer can associate this certificate
with the email address later on, without exposing the email address to other
clients.
*/
func certificateForPublicKey(email string, publicKey *rsa.PublicKey) ([]byte, error) {
	encryptedEmail, err := Encrypt(email)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	template := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(int64(time.Now().Nanosecond())),
		Subject: pkix.Name{
			Organization: []string{"Lantern Network"},
			CommonName:   encryptedEmail,
		},
		NotBefore: now.Add(-1 * ONE_WEEK),
		NotAfter:  now.Add(TWO_WEEKS),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:        true,
	}

	issuerCertificate := certificate
	if issuerCertificate == nil {
		log.Println("We don't have a cert, self-signing using template")
		// Note - for self-signed certificates, we include the host's external IP address
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		issuerCertificate = &template
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, issuerCertificate, publicKey, privateKey)
	if err != nil {
		return nil, err
	}
	return derBytes, nil
}

// saveCertificate() saves our certificate to disk
func saveCertificate(derBytes []byte) {
	certOut, err := os.Create(CertificateFile)
	if err != nil {
		log.Fatalf("Failed to open %s for writing: %s", CertificateFile, err)
	}
	pem.Encode(certOut, &pem.Block{Type: PEM_HEADER_CERTIFICATE, Bytes: derBytes})
	certOut.Close()
	log.Printf("Wrote certificate to %s", CertificateFile)

	certificate, err = x509.ParseCertificate(derBytes)
	if err != nil {
		log.Fatalf("Failed to parse der bytes into Certificate: %s", err)
	}
}
