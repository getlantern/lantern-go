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
	cert, certChannel := keys.Certificate()
	if cert == nil {
		// wait for cert
		cert = <-certChannel
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: keys.TrustedParents,
			Certificates: []tls.Certificate{
				tls.Certificate{
					PrivateKey: keys.PrivateKey(),
					Leaf:       cert,
				},
			},
		},
		DisableCompression: true,
	}
	client = &http.Client{Transport: tr}
	go runLocal()
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
	reqOut, _ := http.NewRequest(r.Method, r.URL.String(), r.Body)
	log.Printf("Processing local request for: %s", reqOut.URL)
	reqOut.Header.Add("X-Original-Host", reqOut.Host)
	reqOut.Header.Add("X-Original-Scheme", reqOut.URL.Scheme)
	// TODO: this needs to come from auto-discovery and statically configured fallback info
	reqOut.Host = "127.0.0.1:16200"
	reqOut.URL.Host = "127.0.0.1:16200"
	reqOut.URL.Scheme = "https"
	log.Print(reqOut)
	resp, _ := client.Do(reqOut)
	// Write headers
	for k, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	// Write data
	io.Copy(w, resp.Body)
	defer r.Body.Close()
}
