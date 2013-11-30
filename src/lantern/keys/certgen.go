/*
This file contains private logic for the keys package that encapsulates an
http-based channel to allow child user nodes to request a certificate from their
parents.

Certificates are requested by POSTing the DER bytes of the child's public key
to https://[parent's signaling address]/mycert.

The parent authenticates the child on the basis of their email address using
Mozilla Persona.  Before requesting a certificate, the child obtains an
identity assertion from Mozilla Persona (see package lantern/persona).  That
identity assertion is then included with the certificate request in the
X-Lantern-Identity header, which the parent then independently verifies with
Mozilla Persona.
*/
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

// PATH at which the parent listens for certificate requests.
const PATH = "/mycert"

// X_LANTERN_IDENTITY is the header that's used to transmit a Mozilla Persona
// identity assertion with certificate requests.
const X_LANTERN_IDENTITY = "X-Lantern-Identity"

// tr is an http transport that trusts this lantern's parent on the basis of
// the certs stored in TrustedParents.
var tr = &http.Transport{
	TLSClientConfig: &tls.Config{RootCAs: TrustedParents},
}

// client uses the tr transport to trust the right parent
var client = &http.Client{Transport: tr}

func init() {
	// Register genCert to handle requests to PATH
	http.HandleFunc(PATH, genCert)
}

// requestCertFromParent() requests a certificate from the parent node for the
// given public key.
func requestCertFromParent(publicKeyBytes []byte) ([]byte, error) {
	// Get our identity assertion (this blocks until the UI flow for getting
	// the identity assertion has finished)
	identityAssertion := <-persona.GetIdentityAssertion()

	// Set up our request to the parent
	url := "https://" + config.ParentAddress() + PATH
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(publicKeyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Add(X_LANTERN_IDENTITY, identityAssertion)

	// Make our request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("http request failed: %s %s", resp.StatusCode, resp.Status)
		}
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return bytes, nil
	}
}

// genCert() handles requests from a child to generate a certificate.
func genCert(resp http.ResponseWriter, req *http.Request) {
	// Always make sure that the request body gets closed
	defer req.Body.Close()

	// helper function for responding to request
	var respond = func(statusCode int, msg string) {
		log.Print(msg)
		resp.WriteHeader(statusCode)
		resp.Write([]byte(msg))
	}

	if assertion := req.Header.Get(X_LANTERN_IDENTITY); assertion == "" {
		respond(400, fmt.Sprintf("Request didn't include a %s header", X_LANTERN_IDENTITY))
	} else {
		if pr, err := persona.ValidateAssertion(assertion); err != nil {
			respond(400, "Identity failed to validate with Mozilla")
		} else {
			if publicKeyBytes, err := ioutil.ReadAll(req.Body); err != nil {
				respond(400, "Request didn't include the public key's bytes")
			} else {
				certBytes, err := certificateForBytes(pr.Email, publicKeyBytes)
				if err != nil {
					respond(500, fmt.Sprintf("Unable to generate certificate: %s", err))
				}
				resp.Header().Set("Content-Type", "application/octet-stream")
				_, err = resp.Write(certBytes)
				if err != nil {
					log.Printf("Unexpected error in returning certificate bytes: %s", err)
					resp.WriteHeader(500)
				}
			}
		}
	}
}
