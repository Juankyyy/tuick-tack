package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"

	"tuick-tack/internal/protocol"
)

// Client is the TUI side of the pipe. Send is safe for concurrent use; read
// messages from a single goroutine.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	mu   sync.Mutex
}

// Dial connects to path and performs the versioned handshake.
func Dial(path string) (*Client, protocol.HelloMessage, error) {
	return DialWithin(path, 2*time.Second)
}

// DialWithin is Dial with an explicit connect timeout. The TUI uses a short
// timeout while waiting for a freshly spawned daemon.
func DialWithin(path string, timeout time.Duration) (*Client, protocol.HelloMessage, error) {
	if timeout <= 0 {
		timeout = time.Second
	}
	conn, err := winio.DialPipe(path, &timeout)
	if err != nil {
		return nil, protocol.HelloMessage{}, err
	}
	c := &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}
	if err := c.Send(protocol.ClientMessage{Type: protocol.TypeHello, Protocol: protocol.Version}); err != nil {
		conn.Close()
		return nil, protocol.HelloMessage{}, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	var raw json.RawMessage
	if err := c.dec.Decode(&raw); err != nil {
		conn.Close()
		return nil, protocol.HelloMessage{}, fmt.Errorf("handshake: %w", err)
	}
	msg, err := protocol.DecodeServer(raw)
	if err != nil {
		conn.Close()
		return nil, protocol.HelloMessage{}, fmt.Errorf("handshake: %w", err)
	}
	if em, ok := msg.(protocol.ErrorMessage); ok {
		conn.Close()
		return nil, protocol.HelloMessage{}, fmt.Errorf("handshake rejected: %s: %s", em.Code, em.Message)
	}
	hello, ok := msg.(protocol.HelloMessage)
	if !ok {
		conn.Close()
		return nil, protocol.HelloMessage{}, errors.New("handshake: unexpected message")
	}
	if hello.Protocol != protocol.Version {
		conn.Close()
		return nil, protocol.HelloMessage{}, fmt.Errorf("handshake: daemon speaks protocol %d, need %d",
			hello.Protocol, protocol.Version)
	}
	return c, hello, nil
}

// Send writes one command.
func (c *Client) Send(msg protocol.ClientMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enc.Encode(msg)
}

// ReadMessage waits for the next daemon message.
func (c *Client) ReadMessage() (protocol.ServerMessage, error) {
	var raw json.RawMessage
	if err := c.dec.Decode(&raw); err != nil {
		return nil, err
	}
	return protocol.DecodeServer(raw)
}

// SetReadDeadline bounds a read, including one already in progress.
func (c *Client) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// Close disconnects from the daemon.
func (c *Client) Close() error {
	return c.conn.Close()
}
