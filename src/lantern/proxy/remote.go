package proxy

import (
	"crypto/tls"
	"io"
	"lantern/config"
	"lantern/keys"
	"log"
	"net/http"
	"time"
)

var httpClient = &http.Client{}
			
func init() {
	go runRemote()
}

func runRemote() {
	cert, certChannel := keys.Certificate()
	if cert == nil {
		// wait for cert
		cert = <-certChannel
	}

	server := &http.Server{
		Addr:         config.RemoteProxyAddress(),
		Handler:      http.HandlerFunc(handleRemoteRequest),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		TLSConfig: &tls.Config{
			ClientCAs:  keys.TrustedParents,
			ClientAuth: tls.RequestClientCert,
		},
	}

	log.Printf("About to start remote proxy at: %s", config.RemoteProxyAddress())
	if err := server.ListenAndServeTLS(keys.CertificateFile, keys.PrivateKeyFile); err != nil {
		log.Fatalf("Unable to start remote proxy: %s", err)
	}
}

func handleRemoteRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	peerCertificates := r.TLS.PeerCertificates
	if len(peerCertificates) == 0 {
		log.Printf("No peer certificates provided")
	} else {
		peerCertificate := peerCertificates[0]
		if email, err := keys.Decrypt(peerCertificate.Subject.CommonName); err != nil {
			log.Printf("Unable to decrypt email: %s", err)
		} else {
			// TODO: check email?  Maybe this is only needed for the signaling channel
			log.Printf("Peer Email is: %s", email)
			if reqOut, err := http.NewRequest(r.Method, r.URL.String(), r.Body); err != nil {
				log.Printf("Error creating request: %s", err)
			} else {
				reqOut.Host = r.Header.Get("X-Original-Host")
				reqOut.URL.Host = r.Header.Get("X-Original-Host")
				reqOut.URL.Scheme = r.Header.Get("X-Original-Scheme")
				log.Printf("Processing remote request for: %s", reqOut.URL)
				if resp, err := httpClient.Do(reqOut); err != nil {
					log.Printf("Error issuing request: %s", err)
				} else {
					// Write headers
					for k, values := range resp.Header {
						for _, v := range values {
							w.Header().Add(k, v)
						}
					}
					// Write data
					io.Copy(w, resp.Body)
				}
			}
		}
	}
}
