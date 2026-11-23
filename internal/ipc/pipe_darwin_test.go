//go:build darwin

package ipc_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testPipePath creates the socket under /tmp. t.TempDir lives under TMPDIR
// (/var/folders/...), whose path plus a long test name can exceed the 104-byte
// Unix socket limit on macOS.
func testPipePath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tuick-tack-ipc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "daemon.sock")
}

func rawDial(path string) (net.Conn, error) {
	return net.DialTimeout("unix", path, 2*time.Second)
}
