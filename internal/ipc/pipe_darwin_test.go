//go:build darwin

package ipc_test

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

func testPipePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "daemon.sock")
}

func rawDial(path string) (net.Conn, error) {
	return net.DialTimeout("unix", path, 2*time.Second)
}
