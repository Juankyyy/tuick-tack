//go:build windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

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

// listen opens the named pipe. go-winio creates it with
// FILE_PIPE_REJECT_REMOTE_CLIENTS, so only local processes can connect.
func listen(path string) (net.Listener, error) {
	sd, err := pipeSecurityDescriptor()
	if err != nil {
		return nil, fmt.Errorf("pipe security: %w", err)
	}
	// Explicit buffers: with the default zero sizes a write can block until the
	// client posts a read, which can stall a broadcast or the shutdown notice.
	// 64 KiB holds the 10 Hz snapshots while the TUI is busy rendering.
	return winio.ListenPipe(path, &winio.PipeConfig{
		SecurityDescriptor: sd,
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	})
}

func dial(path string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(path, &timeout)
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
	// Protected DACL: SYSTEM and the current user only.
	return fmt.Sprintf("D:P(A;;GA;;;SY)(A;;GA;;;%s)", user.User.Sid.String()), nil
}
