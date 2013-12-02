package proxy

import (
	"io"
	"lantern/config"
	"lantern/keys"
	"log"
	"net/http"
	"time"
)

func init() {
	go runRemote()
}

func runRemote() {
	server := &http.Server{
		Addr:         config.RemoteProxyAddress(),
		Handler:      http.HandlerFunc(handleRemoteRequest),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("About to start remote proxy at: %s", config.RemoteProxyAddress())
	if err := server.ListenAndServeTLS(keys.CertificateFile, keys.PrivateKeyFile); err != nil {
		log.Fatalf("Unable to start remote proxy: %s", err)
	}
}

func handleRemoteRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	client := &http.Client{}
	if reqOut, err := http.NewRequest(r.Method, r.URL.String(), r.Body); err != nil {
		log.Printf("Error creating request: %s", err)
	} else {
		reqOut.Host = r.Header.Get("X-Original-Host")
		reqOut.URL.Host = r.Header.Get("X-Original-Host")
		reqOut.URL.Scheme = r.Header.Get("X-Original-Scheme")
		log.Printf("Processing remote request for: %s", reqOut.URL)
		if resp, err := client.Do(reqOut); err != nil {
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
