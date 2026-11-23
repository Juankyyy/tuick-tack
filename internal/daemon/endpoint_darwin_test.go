//go:build darwin

package daemon

import (
	"path/filepath"
	"testing"
)

// testEndpoint returns a unique Unix socket path for IPC tests. t.TempDir stays
// well below the macOS 104-byte socket path limit.
func testEndpoint(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "daemon.sock")
}
