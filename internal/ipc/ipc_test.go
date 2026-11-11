package ipc_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
)

type fakeBackend struct {
	mu   sync.Mutex
	cfg  config.Config
	st   engine.State
	srv  *ipc.Server
	cmds chan protocol.ClientMessage
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		cfg:  config.Default(),
		st:   engine.State{Status: engine.Idle},
		cmds: make(chan protocol.ClientMessage, 64),
	}
}

func (b *fakeBackend) Config() config.Config {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cfg
}

func (b *fakeBackend) Snapshot() engine.State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.st
}

func (b *fakeBackend) Command(c *ipc.Session, msg protocol.ClientMessage) {
	select {
	case b.cmds <- msg:
	default:
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch msg.Type {
	case protocol.TypeStart:
		b.st = engine.State{
			Status:    engine.Running,
			Duration:  time.Duration(msg.Seconds) * time.Second,
			Remaining: time.Duration(msg.Seconds) * time.Second,
			Label:     msg.Label,
			PresetID:  msg.PresetID,
		}
		b.srv.BroadcastState(b.st)
	case protocol.TypePause:
		b.st.Status = engine.Paused
		b.srv.BroadcastState(b.st)
	case protocol.TypeGetState:
		c.Send(protocol.NewState(b.st))
	case protocol.TypePresetAdd:
		b.cfg.Presets = append(b.cfg.Presets, config.Preset{ID: "new", Label: msg.Label, Seconds: msg.Seconds})
		b.srv.BroadcastConfig(b.cfg)
	}
}

var pipeCounter atomic.Uint64

func testPipePath(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`\\.\pipe\tuick-tack-test-%d-%d`, os.Getpid(), pipeCounter.Add(1))
}

func startServer(t *testing.T, path string) (*ipc.Server, *fakeBackend) {
	t.Helper()
	b := newFakeBackend()
	srv := ipc.NewServer(path, "dev", b)
	b.srv = srv
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, b
}

func dial(t *testing.T, path string) (*ipc.Client, protocol.HelloMessage) {
	t.Helper()
	c, hello, err := ipc.Dial(path)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, hello
}

func readUntil(t *testing.T, c *ipc.Client, want string, pred func(protocol.ServerMessage) bool) protocol.ServerMessage {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = c.SetReadDeadline(deadline)
		msg, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage waiting for %s: %v", want, err)
		}
		if pred(msg) {
			return msg
		}
	}
	t.Fatalf("timed out waiting for %s", want)
	return nil
}

func TestHandshakeAndBroadcast(t *testing.T) {
	path := testPipePath(t)
	_, _ = startServer(t, path)

	c1, hello1 := dial(t, path)
	c2, hello2 := dial(t, path)
	for _, h := range []protocol.HelloMessage{hello1, hello2} {
		if h.Type != protocol.TypeHello || h.Protocol != protocol.Version ||
			h.AppVersion != "dev" || len(h.Config.Presets) != 3 {
			t.Fatalf("unexpected hello: %+v", h)
		}
	}
	for _, c := range []*ipc.Client{c1, c2} {
		msg := readUntil(t, c, "initial state", func(m protocol.ServerMessage) bool {
			_, ok := m.(protocol.StateMessage)
			return ok
		})
		if st := msg.(protocol.StateMessage); st.Status != "idle" {
			t.Fatalf("initial state = %+v", st)
		}
	}

	if err := c1.Send(protocol.ClientMessage{Type: protocol.TypeStart, Seconds: 300, Label: "5 min", PresetID: "p5"}); err != nil {
		t.Fatalf("Send start: %v", err)
	}
	for _, c := range []*ipc.Client{c1, c2} {
		msg := readUntil(t, c, "running state", func(m protocol.ServerMessage) bool {
			st, ok := m.(protocol.StateMessage)
			return ok && st.Status == "running"
		})
		st := msg.(protocol.StateMessage)
		if st.DurationMS != 300000 || st.RemainingMS != 300000 || st.Label != "5 min" {
			t.Fatalf("running state = %+v", st)
		}
	}

	if err := c1.Send(protocol.ClientMessage{Type: protocol.TypePresetAdd, Label: "Focus", Seconds: 1500}); err != nil {
		t.Fatalf("Send preset_add: %v", err)
	}
	for _, c := range []*ipc.Client{c1, c2} {
		msg := readUntil(t, c, "config", func(m protocol.ServerMessage) bool {
			_, ok := m.(protocol.ConfigMessage)
			return ok
		})
		cm := msg.(protocol.ConfigMessage)
		if len(cm.Presets) != 4 || cm.Presets[3].Label != "Focus" {
			t.Fatalf("config = %+v", cm)
		}
	}
}

func TestCommandRepliesAndValidation(t *testing.T) {
	path := testPipePath(t)
	_, _ = startServer(t, path)
	c, _ := dial(t, path)

	if err := c.Send(protocol.ClientMessage{Type: protocol.TypeGetState}); err != nil {
		t.Fatal(err)
	}
	readUntil(t, c, "state reply", func(m protocol.ServerMessage) bool {
		_, ok := m.(protocol.StateMessage)
		return ok
	})

	if err := c.Send(protocol.ClientMessage{Type: protocol.TypeStart, Seconds: 0, Label: "bad"}); err != nil {
		t.Fatal(err)
	}
	msg := readUntil(t, c, "validation error", func(m protocol.ServerMessage) bool {
		_, ok := m.(protocol.ErrorMessage)
		return ok
	})
	if em := msg.(protocol.ErrorMessage); em.Code != protocol.CodeInvalidDuration {
		t.Fatalf("error = %+v", em)
	}

	if err := c.Send(protocol.ClientMessage{Type: "wat"}); err != nil {
		t.Fatal(err)
	}
	msg = readUntil(t, c, "unknown command error", func(m protocol.ServerMessage) bool {
		_, ok := m.(protocol.ErrorMessage)
		return ok
	})
	if em := msg.(protocol.ErrorMessage); em.Code != protocol.CodeInvalidCommand {
		t.Fatalf("error = %+v", em)
	}
}

func TestProtocolMismatch(t *testing.T) {
	path := testPipePath(t)
	_, _ = startServer(t, path)

	timeout := 2 * time.Second
	conn, err := winio.DialPipe(path, &timeout)
	if err != nil {
		t.Fatalf("dial raw: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if err := json.NewEncoder(conn).Encode(protocol.ClientMessage{Type: protocol.TypeHello, Protocol: protocol.Version + 1}); err != nil {
		t.Fatal(err)
	}
	var raw json.RawMessage
	if err := json.NewDecoder(conn).Decode(&raw); err != nil {
		t.Fatalf("read rejection: %v", err)
	}
	msg, err := protocol.DecodeServer(raw)
	if err != nil {
		t.Fatal(err)
	}
	em, ok := msg.(protocol.ErrorMessage)
	if !ok || em.Code != protocol.CodeUnsupportedProtocol {
		t.Fatalf("rejection = %#v", msg)
	}
}

func TestReconnectAndClose(t *testing.T) {
	path := testPipePath(t)
	srv, _ := startServer(t, path)
	c1, _ := dial(t, path)
	if err := srv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// The old client observes a closed pipe.
	_ = c1.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := c1.ReadMessage(); err == nil {
		t.Fatal("expected read error after server close")
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// A new server on the same path accepts a fresh client.
	srv2, _ := startServer(t, path)
	c2, hello := dial(t, path)
	if hello.Protocol != protocol.Version {
		t.Fatalf("unexpected hello after reconnect: %+v", hello)
	}
	if err := c2.Send(protocol.ClientMessage{Type: protocol.TypeStart, Seconds: 60, Label: "one"}); err != nil {
		t.Fatal(err)
	}
	readUntil(t, c2, "state after reconnect", func(m protocol.ServerMessage) bool {
		st, ok := m.(protocol.StateMessage)
		return ok && st.Status == "running"
	})
	if err := srv2.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestServerPathUnique(t *testing.T) {
	path := testPipePath(t)
	srv, _ := startServer(t, path)
	defer srv.Close()
	b2 := newFakeBackend()
	srv2 := ipc.NewServer(path, "dev", b2)
	if err := srv2.Start(); err == nil {
		srv2.Close()
		t.Fatal("second server on the same pipe path should fail")
	}
}

func TestDialMissingDaemon(t *testing.T) {
	path := testPipePath(t)
	if c, _, err := ipc.Dial(path); err == nil {
		c.Close()
		t.Fatal("expected dial to a missing daemon to fail")
	}
}
