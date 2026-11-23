//go:build windows

package daemon

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
	return fmt.Sprintf(`\\.\pipe\tuick-tack-test-%d-%d`, os.Getpid(), endpointCounter.Add(1))
}
