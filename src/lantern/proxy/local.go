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

var client *http.Client

func init() {
	x509cert, certChannel := keys.Certificate()
	if x509cert == nil {
		// wait for cert
		x509cert = <-certChannel
	}

	if cert, err := tls.LoadX509KeyPair(keys.CertificateFile, keys.PrivateKeyFile); err != nil {
		log.Fatalf("Unable to load x509 key pair: %s", err)
	} else {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      keys.TrustedParents,
				Certificates: []tls.Certificate{cert},
			},
		}
		client = &http.Client{Transport: tr}
		go runLocal()
	}
}

func runLocal() {
	server := &http.Server{
		Addr:         config.LocalProxyAddress(),
		Handler:      http.HandlerFunc(handleLocalRequest),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("About to start local proxy at: %s", config.LocalProxyAddress())
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Unable to start local proxy: %s", err)
	}
}

func handleLocalRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	reqOut, _ := http.NewRequest(r.Method, r.URL.String(), r.Body)
	log.Printf("Processing local request for: %s", reqOut.URL)
	reqOut.Header.Add("X-Original-Host", reqOut.Host)
	reqOut.Header.Add("X-Original-Scheme", reqOut.URL.Scheme)
	// TODO: this needs to come from auto-discovery and statically configured fallback info
	reqOut.Host = "127.0.0.1:16200"
	reqOut.URL.Host = "127.0.0.1:16200"
	reqOut.URL.Scheme = "https"
	if resp, err := client.Do(reqOut); err != nil {
		log.Printf("Problem submitting request to remote proxy: %s", err)
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
