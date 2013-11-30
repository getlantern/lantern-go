package keys

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"lantern/config"
	"lantern/persona"
	"log"
	"net/http"
)

const PATH = "/mycert"
const X_LANTERN_IDENTITY = "X-Lantern-Identity"

var tr = &http.Transport{
	TLSClientConfig:    &tls.Config{RootCAs: TrustedParents},
	DisableCompression: true,
}

var client = &http.Client{Transport: tr}

func init() {
	http.HandleFunc(PATH, genCert)
}

func requestCertFromParent(identityAssertion string, publicKeyBytes []byte) ([]byte, error) {
	url := "https://" + config.ParentAddress() + PATH
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(publicKeyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Add(X_LANTERN_IDENTITY, identityAssertion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Failed to get cert from parent: %s %s", resp.StatusCode, resp.Status)
		}
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return bytes, nil
	}
}

func genCert(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// helper function for responding to request
	var respond = func(statusCode int, msg string) {
		log.Print(msg)
		w.WriteHeader(statusCode)
		w.Write([]byte(msg))
	}

	if assertion := r.Header.Get(X_LANTERN_IDENTITY); assertion == "" {
		respond(400, fmt.Sprintf("Request didn't include a %s header", X_LANTERN_IDENTITY))
	} else {
		if pr, err := persona.ValidateAssertion(assertion); err != nil {
			respond(400, "Identity failed to validate with Mozilla")
		} else {
			if publicKeyBytes, err := ioutil.ReadAll(r.Body); err != nil {
				respond(400, "Request didn't include the public key's bytes")
			} else {
				certBytes, err := certificateForBytes(pr.Email, publicKeyBytes)
				if err != nil {
					respond(500, fmt.Sprintf("Unable to generate certificate: %s", err))
				}
				w.Header().Set("Content-Type", "application/octet-stream")
				_, err = w.Write(certBytes)
				if err != nil {
					log.Printf("Unexpected error in returning certificate bytes: %s", err)
					w.WriteHeader(500)
				}
			}
		}
	}
}
