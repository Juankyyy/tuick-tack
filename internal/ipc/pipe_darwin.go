//go:build darwin

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// unixSocketLimit bounds the socket path. macOS rejects paths longer than 104
// bytes with an obscure ENAMETOOLONG; fail early with a clear error instead.
const unixSocketLimit = 100

// PipePath returns the per-user socket path. The per-user cache directory is
// already private to the account; the uid in the name keeps a stale socket from
// a different account from being reused if the directory is ever shared.
func PipePath() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(cache, "tuick-tack", fmt.Sprintf("daemon-%d.sock", os.Getuid()))
	if len(path) > unixSocketLimit {
		return "", fmt.Errorf("ipc: socket path is too long: %s", path)
	}
	return path, nil
}

// listen opens the Unix socket. A socket left behind by a crashed daemon is
// removed when nothing accepts a connection on it; a live daemon keeps its
// socket and the second server fails to start. The socket is mode 0600.
func listen(path string) (net.Listener, error) {
	if len(path) > unixSocketLimit {
		return nil, fmt.Errorf("socket path is %d bytes, the limit is %d: %s", len(path), unixSocketLimit, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		conn, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			return nil, errors.New("socket is already in use")
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, removeErr
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

func dial(path string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.Dial("unix", path)
}
