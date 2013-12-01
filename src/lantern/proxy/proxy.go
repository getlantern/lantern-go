package proxy

import (
	"lantern/config"
	"log"
	"net/http"
	"time"
)

func init() {
	go run()
}

func run() {
	server := &http.Server{
		Addr:         config.LocalProxyAddress(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Print(r)
		defer r.Body.Close()
	})

	log.Print("About to start local proxy")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Unable to start local proxy: %s", err)
	}
}
