package signaling

import (
	"code.google.com/p/go.net/websocket"
	"crypto/tls"
	"encoding/json"
	"lantern/config"
	"lantern/keys"
	"log"
	"net/url"
)

var ws *websocket.Conn

func sendToParent(msg Message) {
	ensureConnected()
	if !config.IsRootNode() {
		if bytes, err := json.Marshal(msg); err != nil {
			log.Printf("Unable to marchasl message to JSON! {}", err)
		} else {
			ws.Write(bytes)
		}
	}
}

func ensureConnected() {
	if ws == nil {
		var err error
		wsConfig := &websocket.Config{
			TlsConfig: &tls.Config{RootCAs: keys.TrustedParents},
		}
		wsConfig.Location, err = url.Parse("wss://" + config.ParentAddress() + "/")
		if err != nil {
			log.Fatalf("Unable to parse server url: {}", err)
		}
		wsConfig.Origin, err = url.Parse("https://127.0.0.1")
		if err != nil {
			log.Fatalf("Unable to parse server url: {}", err)
		}
		wsConfig.Version = websocket.ProtocolVersionHybi13
		if ws, err = websocket.DialConfig(wsConfig); err != nil {
			log.Fatalf("Unable to connect to signaling channel to parent: {}", err)
		}
	}
}
