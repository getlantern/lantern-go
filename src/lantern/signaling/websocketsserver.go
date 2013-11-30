package signaling

import (
	"fmt"
	"io"
	"lantern/util"
	"log"
	"net/http"

	"code.google.com/p/go.net/websocket"
)

const channelBufSize = 100

var maxId int = 0

/*
ClientConnection represents a client connecting via a WebSocket.

Adapted from here:
https://github.com/golang-samples/websocket/tree/master/websocket-chat
*/
type ClientConnection struct {
	id     int
	ws     *websocket.Conn
	server *Server
	ch     chan *Message
	doneCh chan bool
	emails util.StringSet
}

// Create new ClientConnection.
func NewClientConnection(ws *websocket.Conn, server *Server) *ClientConnection {

	if ws == nil {
		panic("ws cannot be nil")
	}

	if server == nil {
		panic("server cannot be nil")
	}

	maxId++
	ch := make(chan *Message, channelBufSize)
	doneCh := make(chan bool)

	return &ClientConnection{
		id:     maxId,
		ws:     ws,
		server: server,
		ch:     ch,
		doneCh: doneCh}
}

func (c *ClientConnection) Conn() *websocket.Conn {
	return c.ws
}

func (c *ClientConnection) Write(msg *Message) {
	select {
	case c.ch <- msg:
	default:
		c.server.Del(c)
		err := fmt.Errorf("ClientConnection %d is disconnected.", c.id)
		c.server.Err(err)
	}
}

func (c *ClientConnection) Done() {
	c.doneCh <- true
}

// Listen Write and Read request via chanel
func (c *ClientConnection) Listen() {
	go c.listenWrite()
	c.listenRead()
}

// Listen write request via chanel
func (c *ClientConnection) listenWrite() {
	log.Println("Listening write to ClientConnection")
	for {
		select {

		// send message to the ClientConnection
		case msg := <-c.ch:
			log.Println("Send:", msg)
			websocket.JSON.Send(c.ws, msg)

		// receive done request
		case <-c.doneCh:
			c.server.Del(c)
			c.doneCh <- true // for listenRead method
			return
		}
	}
}

// Listen read request via channel
func (c *ClientConnection) listenRead() {
	log.Println("Listening read from ClientConnection")
	for {
		select {

		// receive done request
		case <-c.doneCh:
			c.server.Del(c)
			c.doneCh <- true // for listenWrite method
			return

		// read data from websocket connection
		default:
			var msg Message
			err := websocket.JSON.Receive(c.ws, &msg)
			if err == io.EOF {
				c.doneCh <- true
			} else if err != nil {
				c.server.Err(err)
			} else {
				switch msg.T {
				case TYPE_REGISTRATION:
					c.emails.Add(msg.D)
				case TYPE_DEREGISTRATION:
					c.emails.Remove(msg.D)
				default:
					c.server.SendAll(&msg)
				}
			}
		}
	}
}

// Server.
type Server struct {
	messages  []*Message
	clients   map[int]*ClientConnection
	addCh     chan *ClientConnection
	delCh     chan *ClientConnection
	sendAllCh chan *Message
	doneCh    chan bool
	errCh     chan error
}

// Create new server.
func NewServer() *Server {
	messages := []*Message{}
	clients := make(map[int]*ClientConnection)
	addCh := make(chan *ClientConnection)
	delCh := make(chan *ClientConnection)
	sendAllCh := make(chan *Message)
	doneCh := make(chan bool)
	errCh := make(chan error)

	return &Server{
		messages,
		clients,
		addCh,
		delCh,
		sendAllCh,
		doneCh,
		errCh,
	}
}

func (s *Server) Add(c *ClientConnection) {
	s.addCh <- c
}

func (s *Server) Del(c *ClientConnection) {
	s.delCh <- c
}

func (s *Server) SendAll(msg *Message) {
	s.sendAllCh <- msg
}

func (s *Server) Done() {
	s.doneCh <- true
}

func (s *Server) Err(err error) {
	s.errCh <- err
}

func (s *Server) sendPastMessages(c *ClientConnection) {
	for _, msg := range s.messages {
		c.Write(msg)
	}
}

func (s *Server) sendAll(msg *Message) {
	for _, c := range s.clients {
		c.Write(msg)
	}
}

// Listen and serve.
// It serves client connection and broadcast request.
func (s *Server) Listen() {

	log.Println("Listening server...")

	// websocket handler
	onConnected := func(ws *websocket.Conn) {
		defer func() {
			err := ws.Close()
			if err != nil {
				s.errCh <- err
			}
		}()

		client := NewClientConnection(ws, s)
		s.Add(client)
		client.Listen()
	}
	http.Handle("/", websocket.Handler(onConnected))
	log.Println("Created handler")

	for {
		select {

		// Add new a client
		case c := <-s.addCh:
			log.Println("Added new client")
			s.clients[c.id] = c
			log.Println("Now", len(s.clients), "clients connected.")
			s.sendPastMessages(c)

		// del a client
		case c := <-s.delCh:
			log.Println("Delete client")
			delete(s.clients, c.id)

		// broadcast message for all clients
		case msg := <-s.sendAllCh:
			log.Println("Send all:", msg)
			s.messages = append(s.messages, msg)
			s.sendAll(msg)

		case err := <-s.errCh:
			log.Println("Error:", err.Error())

		case <-s.doneCh:
			return
		}
	}
}
