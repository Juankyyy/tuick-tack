//go:build windows

package ipc_test

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

var pipeCounter atomic.Uint64

func testPipePath(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`\\.\pipe\tuick-tack-test-%d-%d`, os.Getpid(), pipeCounter.Add(1))
}

func rawDial(path string) (net.Conn, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(path, &timeout)
}
