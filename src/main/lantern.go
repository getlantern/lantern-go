package main

import (
	"lantern/config"
	"lantern/keys"
	_ "lantern/signaling"
	"log"
	"net/http"
	"runtime"
	"time"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GOMAXPROCS(1)

	bytes := make([]byte, 500)
	for i := 0; i < len(bytes); i++ {
		bytes[i] = 100
	}

	server := &http.Server{
		Addr:         config.RemoteProxyAddress(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Write(bytes)
		//log.Printf("Peer certificates: %s", r.TLS.PeerCertificates)
	})

	log.Print("About to listen")
	if err := server.ListenAndServeTLS(keys.CertificateFile, keys.PrivateKeyFile); err != nil {
		log.Fatalf("Unable to listen: %s", err)
	}
	//	if err := server.ListenAndServe(); err != nil {
	//		log.Fatalf("Unable to listen: %s", err)
	//	}
}
