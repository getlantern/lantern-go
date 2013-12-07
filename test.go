package main

import (
	"log"
	"net/http"
	"runtime"
	"time"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GOMAXPROCS(1)

	bytes := make([]byte, 1)
	for i := 0; i < len(bytes); i++ {
		bytes[i] = 100
	}

	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes)
	})

	log.Print("About to listen")
	if err := server.ListenAndServeTLS("certificate.pem", "privatekey.pem"); err != nil {
		log.Fatalf("Unable to listen: %s", err)
	}
}
