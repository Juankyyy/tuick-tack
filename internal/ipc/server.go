// Package ipc implements the local transport between the TUI and the daemon:
// a named pipe on Windows and a Unix domain socket on macOS. The JSON protocol
// and the connection handling are shared.
package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/protocol"
)

// Backend is the daemon side of the connection.
type Backend interface {
	// Config returns the current configuration for the handshake.
	Config() config.Config
	// Snapshot returns the current countdown.
	Snapshot() engine.State
	// Update returns the latest release the daemon knows about, or nil.
	Update() *protocol.UpdatePayload
	// Command handles a validated client command. The client is provided for
	// replies such as get_state.
	Command(*Session, protocol.ClientMessage)
}

// Server accepts clients and broadcasts state/config messages.
type Server struct {
	path       string
	version    string
	backend    Backend
	withWindow bool

	ln         net.Listener
	mu         sync.Mutex
	clients    map[*Session]struct{}
	closed     bool
	wg         sync.WaitGroup
	acceptDone chan struct{}
}

// flushTimeout bounds how long a shutdown waits for one client write.
const flushTimeout = 250 * time.Millisecond

// outbound is one message queued for a session writer. ack, when non-nil, is
// closed after the write so a shutdown can wait for the notice to leave the
// server before closing the pipe.
type outbound struct {
	data []byte
	ack  chan struct{}
}

// NewServer creates a server for path. withWindow makes the handshake carry the
// TUI window placement; macOS omits it because the app does not move windows.
// Start must be called before Dial.
func NewServer(path, appVersion string, backend Backend, withWindow bool) *Server {
	return &Server{
		path:       path,
		version:    appVersion,
		backend:    backend,
		withWindow: withWindow,
		clients:    make(map[*Session]struct{}),
		acceptDone: make(chan struct{}),
	}
}

// Start opens the transport and starts accepting clients in a background
// goroutine.
func (s *Server) Start() error {
	ln, err := listen(s.path)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.path, err)
	}
	s.ln = ln
	go s.acceptLoop()
	return nil
}

// Path returns the pipe or socket path.
func (s *Server) Path() string { return s.path }

func (s *Server) acceptLoop() {
	defer close(s.acceptDone)
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	dec := json.NewDecoder(conn)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	hello, err := protocol.DecodeClient(raw)
	if err != nil || hello.Type != protocol.TypeHello {
		writeDirect(conn, protocol.NewError(protocol.CodeInvalidMessage, "expected hello"))
		conn.Close()
		return
	}
	if hello.Protocol != protocol.Version {
		writeDirect(conn, protocol.NewError(protocol.CodeUnsupportedProtocol,
			fmt.Sprintf("protocol %d is not supported (need %d)", hello.Protocol, protocol.Version)))
		conn.Close()
		return
	}

	c := &Session{
		server: s,
		conn:   conn,
		dec:    dec,
		send:   make(chan outbound, 256),
		done:   make(chan struct{}),
	}

	// Queue the handshake and the first snapshot before the writer starts, so
	// they are the first two messages the client sees.
	helloReply := protocol.NewHello(s.version, s.backend.Config(), s.backend.Update(), s.withWindow)
	c.enqueue(helloReply)
	c.enqueue(protocol.NewState(s.backend.Snapshot()))

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		c.close()
		return
	}
	s.wg.Add(2)
	s.clients[c] = struct{}{}
	s.mu.Unlock()

	// The release check can finish between building the handshake and adding the
	// client to the broadcast set. Re-read it so that window cannot hide the
	// notice until the next check.
	if helloReply.Update == nil {
		if upd := s.backend.Update(); upd != nil {
			c.enqueue(protocol.NewUpdate(*upd))
		}
	}

	go func() {
		defer s.wg.Done()
		c.writeLoop()
	}()
	go func() {
		defer s.wg.Done()
		c.readLoop()
	}()
}

func writeDirect(conn net.Conn, msg protocol.ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = conn.Write(data)
}

func (s *Server) remove(c *Session) {
	s.mu.Lock()
	delete(s.clients, c)
	s.mu.Unlock()
}

// Broadcast sends msg to every connected client.
func (s *Server) Broadcast(msg protocol.ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	data = append(data, '\n')

	s.mu.Lock()
	clients := make([]*Session, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()
	for _, c := range clients {
		c.enqueueBytes(data)
	}
}

// BroadcastState sends a countdown snapshot.
func (s *Server) BroadcastState(st engine.State) {
	s.Broadcast(protocol.NewState(st))
}

// BroadcastConfig sends the presets, theme, background, sound selection and
// volume.
func (s *Server) BroadcastConfig(c config.Config) {
	s.Broadcast(protocol.NewConfig(c))
}

// Close stops accepting, disconnects clients and waits for their goroutines.
func (s *Server) Close() error { return s.closeWith(nil) }

// CloseWithNotice writes a final message to every connected client and waits
// briefly for the write before closing the pipe. A client can act on the
// notice; Close alone is indistinguishable from a lost connection.
func (s *Server) CloseWithNotice(msg protocol.ServerMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return s.Close()
	}
	return s.closeWith(append(data, '\n'))
}

func (s *Server) closeWith(notice []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	clients := make([]*Session, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	var err error
	if s.ln != nil {
		err = s.ln.Close()
	}
	<-s.acceptDone
	if len(notice) > 0 {
		for _, c := range clients {
			c.sendSync(notice, flushTimeout)
		}
	}
	for _, c := range clients {
		c.close()
	}
	s.wg.Wait()
	return err
}

// Session is a server-side connection. It is safe to use Send from any
// goroutine.
type Session struct {
	server *Server
	conn   net.Conn
	dec    *json.Decoder
	send   chan outbound
	done   chan struct{}
	once   sync.Once
}

func (c *Session) readLoop() {
	defer c.close()
	for {
		var raw json.RawMessage
		if err := c.dec.Decode(&raw); err != nil {
			return
		}
		msg, err := protocol.DecodeClient(raw)
		if err != nil {
			c.enqueue(protocol.NewError(protocol.CodeInvalidMessage, err.Error()))
			return
		}
		if errMsg := msg.Validate(); errMsg != nil {
			c.enqueue(*errMsg)
			continue
		}
		c.server.backend.Command(c, msg)
	}
}

func (c *Session) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case out := <-c.send:
			_, err := c.conn.Write(out.data)
			if out.ack != nil {
				close(out.ack)
			}
			if err != nil {
				c.close()
				return
			}
		}
	}
}

func (c *Session) enqueue(msg protocol.ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	data = append(data, '\n')
	c.enqueueBytes(data)
}

func (c *Session) enqueueBytes(data []byte) {
	c.queue(outbound{data: data})
}

func (c *Session) queue(out outbound) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.send <- out:
	default:
		// A client that cannot keep up reconnects and resynchronizes.
		go c.close()
	}
}

// sendSync queues data and waits until the writer has written it. It reports
// false when the session is closing or the timeout expires; a shutdown uses it
// to deliver its final notice before closing the pipe.
func (c *Session) sendSync(data []byte, timeout time.Duration) bool {
	ack := make(chan struct{})
	select {
	case c.send <- outbound{data: data, ack: ack}:
	case <-c.done:
		return false
	case <-time.After(timeout):
		return false
	}
	select {
	case <-ack:
		return true
	case <-c.done:
		return false
	case <-time.After(timeout):
		return false
	}
}

// Send queues a message for this client only.
func (c *Session) Send(msg protocol.ServerMessage) {
	c.enqueue(msg)
}

// Close disconnects the client. It is safe to call more than once.
func (c *Session) Close() { c.close() }

func (c *Session) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
		c.server.remove(c)
	})
}
