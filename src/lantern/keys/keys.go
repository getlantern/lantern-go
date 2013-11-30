// Stores keys and certificates, backed by pem encoded files on the file system.
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
	"lantern/persona"
	"log"
	"math/big"
	"net"
	"os"
	"reflect"
	"sync"
	"time"
)

const PEM_HEADER_PRIVATE_KEY = "RSA PRIVATE KEY"
const PEM_HEADER_PUBLIC_KEY = "RSA PRIVATE KEY"
const PEM_HEADER_CERTIFICATE = "CERTIFICATE"
const KEY_BITS = 2048
const ONE_WEEK = 7 * 24 * time.Hour
const TWO_WEEKS = ONE_WEEK * 2

var directory string
var PrivateKeyFile string
var privateKey *rsa.PrivateKey
var CertificateFile string
var certificate *x509.Certificate
var parentCertFile string
var TrustedParents = x509.NewCertPool()
var certMutex sync.RWMutex
var waitingForCerts = make([]chan *x509.Certificate, 0)

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

func init() {
	log.Print("Configuring keystore")
	ownPath := config.ConfigDir + "/keys/own/"
	trustedPath := config.ConfigDir + "/keys/trusted/"
	PrivateKeyFile = ownPath + "privatekey.pem"
	CertificateFile = ownPath + "certificate.pem"
	parentCertFile = trustedPath + "parentcert.pem"
	if err := os.MkdirAll(ownPath, 0755); err != nil {
		log.Fatalf("Unable to create directory for own keys '%s': %s", ownPath, err)
	}
	if config.ParentAddress() != "" {
		loadParentCert()
	}
	loadPrivateKey()
	loadCertificate()
}

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
}

func initCertificate() {
	var derBytes []byte
	var err error
	if config.ParentAddress() == "" {
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
		assertion := <-persona.GetIdentityAssertion()
		derBytes, err = requestCertFromParent(assertion, publicKeyBytes)
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

// createCertificate creates a certificate from the public key's DER bytes,
// returning DER bytes for the Certificate.  The supplied email is encrypted and
// stored as the common name so that the issuer can associate the certificate
// with the email address later on.
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

func certificateForPublicKey(email string, publicKey *rsa.PublicKey) ([]byte, error) {
	encryptedEmail, err := encrypt(email)
	if err != nil {
		return nil, err
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(TWO_WEEKS)

	template := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(int64(time.Now().Nanosecond())),
		Subject: pkix.Name{
			Organization: []string{"Lantern Network"},
			CommonName:   encryptedEmail,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:        true,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, privateKey)
	if err != nil {
		return nil, err
	}
	return derBytes, nil
}

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

func encrypt(value string) (string, error) {
	if bytes, err := rsa.EncryptPKCS1v15(rand.Reader, &(privateKey.PublicKey), []byte(value)); err != nil {
		return "", err
	} else {
		return base64.StdEncoding.EncodeToString(bytes), nil
	}
}
