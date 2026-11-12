// Package ipc implements the local named-pipe transport between the TUI and
// the daemon.
package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"

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
	// Command handles a validated client command. The client is provided for
	// replies such as get_state.
	Command(*Session, protocol.ClientMessage)
}

// PipePath returns the per-user, per-session pipe name from SPEC §4.
func PipePath() (string, error) {
	user := os.Getenv("USERNAME")
	if user == "" {
		user = "user"
	}
	user = strings.ToLower(user)
	var b strings.Builder
	for _, r := range user {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
				b.WriteByte('_')
			}
		}
		if b.Len() >= 24 {
			break
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		name = "user"
	}

	var session uint32
	_ = windows.ProcessIdToSessionId(uint32(os.Getpid()), &session)
	return fmt.Sprintf(`\\.\pipe\tuick-tack-%s-%d`, name, session), nil
}

func pipeSecurityDescriptor() (string, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	// Protected DACL: SYSTEM and the current user only. go-winio creates the
	// pipe with FILE_PIPE_REJECT_REMOTE_CLIENTS, so remote access is denied.
	return fmt.Sprintf("D:P(A;;GA;;;SY)(A;;GA;;;%s)", user.User.Sid.String()), nil
}

// Server accepts clients and broadcasts state/config messages.
type Server struct {
	path    string
	version string
	backend Backend

	ln         net.Listener
	mu         sync.Mutex
	clients    map[*Session]struct{}
	closed     bool
	wg         sync.WaitGroup
	acceptDone chan struct{}
}

// NewServer creates a server for path. Start must be called before Dial.
func NewServer(path, appVersion string, backend Backend) *Server {
	return &Server{
		path:       path,
		version:    appVersion,
		backend:    backend,
		clients:    make(map[*Session]struct{}),
		acceptDone: make(chan struct{}),
	}
}

// Start opens the pipe and starts accepting clients in a background goroutine.
func (s *Server) Start() error {
	sd, err := pipeSecurityDescriptor()
	if err != nil {
		return fmt.Errorf("pipe security: %w", err)
	}
	ln, err := winio.ListenPipe(s.path, &winio.PipeConfig{SecurityDescriptor: sd})
	if err != nil {
		return fmt.Errorf("listen pipe: %w", err)
	}
	s.ln = ln
	go s.acceptLoop()
	return nil
}

// Path returns the pipe path.
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
		send:   make(chan []byte, 256),
		done:   make(chan struct{}),
	}

	// Queue the handshake and the first snapshot before the writer starts, so
	// they are the first two messages the client sees.
	c.enqueue(protocol.NewHello(s.version, s.backend.Config()))
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

// BroadcastConfig sends the presets, theme, sound selection and volume.
func (s *Server) BroadcastConfig(c config.Config) {
	s.Broadcast(protocol.NewConfig(c))
}

// Close stops accepting, disconnects clients and waits for their goroutines.
func (s *Server) Close() error {
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
	send   chan []byte
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
		case data := <-c.send:
			if _, err := c.conn.Write(data); err != nil {
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
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.send <- data:
	default:
		// A client that cannot keep up reconnects and resynchronizes.
		go c.close()
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
