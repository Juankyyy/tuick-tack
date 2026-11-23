//go:build darwin

package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// testEndpoint returns a unique Unix socket path for IPC tests. It uses /tmp
// because t.TempDir under TMPDIR plus a long test name can exceed the 104-byte
// Unix socket limit on macOS.
func testEndpoint(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tuick-tack-daemon-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "daemon.sock")
}
