package proxy

import (
	"crypto/tls"
	"fmt"
	"lantern/config"
	"lantern/keys"
	"log"
	"net/http"
	"time"
)

var client *http.Client
var tlsConfig *tls.Config

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
		tlsConfig = &tls.Config{
			RootCAs:      keys.TrustedParents,
			Certificates: []tls.Certificate{cert},
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

func handleLocalRequest(resp http.ResponseWriter, req *http.Request) {
	// TODO: this needs to come from auto-discovery and statically configured fallback info
	upstreamProxy := "127.0.0.1:16200"

	if connOut, err := tls.Dial("tcp", upstreamProxy, tlsConfig); err != nil {
		msg := fmt.Sprintf("Unable to open socket to upstream proxy: %s", err)
		respondBadGateway(resp, req, msg)
	} else {
		if connIn, _, err := resp.(http.Hijacker).Hijack(); err != nil {
			msg := fmt.Sprintf("Unable to access underlying connection from client: %s", err)
			respondBadGateway(resp, req, msg)
		} else {
			req.Write(connOut)
			pipe(connIn, connOut)
		}
	}
}
