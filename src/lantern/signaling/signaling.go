/*
Package signaling encapsulates the signaling channel for lantern.

Lantern nodes are organized into a tree which is responsible for passing
presence notifications to the appropriate parties.  The tree consists of two
different kinds of nodes, master nodes and user nodes.

Master nodes are highly trusted nodes (typically operated by the Lantern team)
that provide the messaging backbone for the Lantern network.

User nodes are nodes run by end users for purposes of proxying traffic on behalf
of, or proxying traffic via, the Lantern network.  User nodes are not as trusted
as server nodes, and they are tied to a specific email address.

Both master and user nodes run the same Lantern software, but the way in which
they authenticate against the Lantern network differs.

Let as assume the following tree of lantern nodes.

root
  1
    1.1
    1.2
      1.2.1
      1.2.2 (a@gmail.com)
  2
    2.1
    2.2 (b@yahoo.com)

- 1 is the parent of 1.1
- 1.1 is a child of 1
- 1.2 is a sibling of 1.1
- 1 and 1.2 are ancestors of 1.2.1
- 1.2 and 1.2.1 are descendants of 1
- Children know the address of their parent
- Parents know the address of their children
- Siblings do not know each others addresses
- Ancestors other than parents do not know their descendants' addresses
- Descendants other than children do not know their ancestors' addresses
- Nodes 1.2.2 and 2.2 are user nodes, all the others are master nodes

Message Passing
---------------
Messages contain information about the type of message, the recipient (email
address) and the payload, which is a JSON encoded string.

Children register with their parents to indicate which email addresses they
can deliver.  User nodes can only register to receive messages for their
specific users.  Master nodes can register to receive messages for any user
and do so up the chain of master nodes until the root parent is reached.

In our example, user node 1.2.2 would register with master 1.2 to indicate that
it can deliver messages for a@gmail.com. 1.2 then registers with 1 to indicate
that it can deliver messages for a@gmail.com, and 1 then registers with root to
indicate that it can deliver messages for a@gmail.com.

When a message is sent, it propagates both up the tree and down to children that
have registered for the recipient email address, terminating at the relevant
user node.

For example, let's say that node 2.2 sends a message to a@gmail.com.  This
message will get routed as follows:

2.2 -> 2 -> root -> 1 -> 1.2 -> 1.2.2

Note what the sender, 2.2, doesn't know:

- Whether or not there's even a node running as a@gmail.com
- Which node is actually running as a@gmail.com
- How to reach the node running as a@gmail.com

The signaling mechanism does not store and forward - messages are passed
immediately.  Since this communication proceeds over the network, and since
any given node may or may not be online or reachable at any given moment, the
signaling mechanism should be considered unreliable.  This has several
implications:

- lantern nodes need to be designed to function correctly whether or not
  a message has gotten through.
- because messages like presence notifications may not make it to all intended
  recipients, they should be resent periodically.  This is a good idea anyway
  because user nodes can come on and offline all the time.
  
Also, because of the potential size of the network, messages should be kept
small - this is not a mechanism for transferring large payloads, it's a
mechanism for delivering to-the-point messages.

Trust and Authentication
------------------------
Nodes trust each other based on a scheme that combines PKI and Mozilla Persona.

- All children trust their parents based on a certificate distributed to the
  child via some out-of-band mechanism (e.g. email)
- Parents trust child master nodes based on them presenting a master-level
  certificate signed by the parent.
- Parents initially trust child user nodes based on them presenting a
  Mozilla Persona identity assertion which the parent is able to verify with
  Mozilla Persona.  After the initial authentication of the child, the parent
  issues a certificate to the child that is tied to the child's email address.
  In particular, the CN of the certificate contains the child's email address,
  encrypted by the parent so that only the parent can read it.  On subsequent
  requests to the parent, the child is identified by this certificate.
- Master nodes maintain certificate revocation lists that allow them to revoke
  any certificates that they have previously issued, both to other master nodes
  and to child nodes.

All of the certificate management stuff is implemented by package
lantern/keystore.

All of the Mozilla Persona stuff is implemented by the package lantern/persona.

Communication Protocol
----------------------
Nodes connect to each other via websockets.  Every master node maintains an
https websocket endpoint at a static host:port.  Child nodes connect to this
endpoint.  All messaging, both for master and user nodes, uses this websocket
channel.

Messages are expected to be small (no more than 1KB).
*/
package signaling

import (
	"lantern/config"
	"lantern/keys"
	"log"
	"net/http"
	"time"
)

type MessageType uint8

const (
	TYPE_REGISTRATION   = 1 // registration of a new email address
	TYPE_DEREGISTRATION = 2 // deregistration of an email address
)

type Message struct {
	R string      // the recipient email address
	T MessageType // the type of message
	D string      // the data payload (might be JSON encoded or not)
}

// Channels that receive new messages sent via the signaling bus
var receivers = make([]chan Message, 0)

// Channel for sending messages to the signaling bus
var messages = make(chan Message)

// Channel for receiving requests to register receivers
var registrations = make(chan chan Message)

// SendMessage sends a Message to the Lantern network.
func SendMessage(m Message) {
	messages <- m
}

// ReceiveMessagesAt allows one to register to receive messages through the
// supplied channel.
func ReceiveMessagesAt(receiver chan Message) {
	registrations <- receiver
}

func init() {
	go run()
	go startWebChannel()
	log.Printf("Listening for signaling connections at: %s", config.SignalingAddress())
}

// Goroutine for processing registrations, message sends and message receives
func run() {
	for {
		select {
		case receiver := <-registrations:
			log.Println("Adding message receiver")
			receivers = append(receivers, receiver)
		case m := <-messages:
			log.Printf("Dispatching message: %s", m)
		}
	}
}

func startWebChannel() {
	server := &http.Server{
		Addr:         config.SignalingAddress(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServeTLS(keys.CertificateFile, keys.PrivateKeyFile); err != nil {
		log.Fatalf("Unable to start signaling websocket server: %s", err)
	}
}
