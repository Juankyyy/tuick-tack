package daemon

import (
	"fmt"
	"os/exec"
	"sync"
)

// playback is the afplay process for the sound that is currently playing. The
// shared soundPlayer serializes Prepare/Play/Stop, but the waiter goroutine
// clears the reference after the process exits, so it needs its own lock.
var playback struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

// afplay is part of macOS and plays the scaled WAV without touching the system
// volume.
const afplay = "/usr/bin/afplay"

// playFile starts path asynchronously with afplay. The previous sound, if any,
// is stopped first.
func playFile(path string) error {
	stopPlayback()
	cmd := exec.Command(afplay, path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("afplay: %w", err)
	}
	playback.mu.Lock()
	playback.cmd = cmd
	playback.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		playback.mu.Lock()
		if playback.cmd == cmd {
			playback.cmd = nil
		}
		playback.mu.Unlock()
	}()
	return nil
}

// stopPlayback kills the current afplay process. The waiter goroutine reaps it.
func stopPlayback() {
	playback.mu.Lock()
	cmd := playback.cmd
	playback.cmd = nil
	playback.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
