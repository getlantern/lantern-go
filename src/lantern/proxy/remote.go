package proxy

import (
	"crypto/tls"
	"fmt"
	"lantern/config"
	"lantern/keys"
	"log"
	"net"
	"net/http"
	"strings"
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

func handleRemoteRequest(resp http.ResponseWriter, req *http.Request) {
	peerCertificates := req.TLS.PeerCertificates
	if len(peerCertificates) == 0 {
		log.Printf("No peer certificates provided")
	} else {
		peerCertificate := peerCertificates[0]
		if _, err := keys.Decrypt(peerCertificate.Subject.CommonName); err != nil {
			msg := fmt.Sprintf("Unable to decrypt email: %s", err)
			respondBadGateway(resp, req, msg)
		} else {
			// TODO: check email?  Maybe this is only needed for the signaling channel
			//log.Printf("Peer Email is: %s", email)
			host := hostIncludingPort(req)
			if connOut, err := net.Dial("tcp", host); err != nil {
				msg := fmt.Sprintf("Unable to open socket to server: %s", err)
				respondBadGateway(resp, req, msg)
			} else {
				if connIn, _, err := resp.(http.Hijacker).Hijack(); err != nil {
					msg := fmt.Sprintf("Unable to access underlying connection from downstream proxy: %s", err)
					respondBadGateway(resp, req, msg)
				} else {
					if req.Method == "CONNECT" {
						connIn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
					} else {
						req.Write(connOut)
					}
					pipe(connIn, connOut)
				}
			}
		}
	}
}

func hostIncludingPort(req *http.Request) (host string) {
	host = req.Host
	if !strings.Contains(host, ":") {
		if req.Method == "CONNECT" {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}
	return
}
