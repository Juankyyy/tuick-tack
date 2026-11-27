//go:build windows

package tui

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
)

var endpointCounter atomic.Uint64

// testEndpoint returns a unique named pipe for IPC tests.
func testEndpoint(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`\\.\pipe\tuick-tack-tui-test-%d-%d`, os.Getpid(), endpointCounter.Add(1))
}
