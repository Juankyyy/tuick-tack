package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"tuick-tack/assets"
)

const (
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
	sndFilename  = 0x00020000
)

var playSoundProc = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

// soundPlayer extracts the embedded chime once and plays it through the Win32
// PlaySoundW API. Failures are non-fatal: the countdown still finishes and the
// tray alerts visually.
type soundPlayer struct {
	mu   sync.Mutex
	path string
}

// newSoundPlayer writes assets/chime.wav to the cache directory when it is
// missing or differs, then returns a player. A zero player is usable and just
// does nothing.
func newSoundPlayer(cacheDir string) (*soundPlayer, error) {
	if cacheDir == "" {
		return &soundPlayer{}, errors.New("no cache directory")
	}
	path := filepath.Join(cacheDir, "chime.wav")
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, assets.ChimeWAV) {
		if err := os.MkdirAll(cacheDir, 0o700); err != nil {
			return &soundPlayer{}, err
		}
		tmp, err := os.CreateTemp(cacheDir, ".chime-*.tmp")
		if err != nil {
			return &soundPlayer{}, err
		}
		name := tmp.Name()
		defer os.Remove(name)
		if _, err := tmp.Write(assets.ChimeWAV); err != nil {
			tmp.Close()
			return &soundPlayer{}, err
		}
		if err := tmp.Close(); err != nil {
			return &soundPlayer{}, err
		}
		if err := os.Rename(name, path); err != nil {
			return &soundPlayer{}, err
		}
	}
	return &soundPlayer{path: path}, nil
}

// Play starts the chime asynchronously. PlaySoundW returns immediately.
func (p *soundPlayer) Play() error {
	if p == nil || p.path == "" {
		return errors.New("sound is not available")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return playFile(p.path)
}

// Stop silences a chime that is still playing.
func (p *soundPlayer) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _, _ = playSoundProc.Call(0, 0, 0)
}

func playFile(path string) error {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	res, _, callErr := playSoundProc.Call(
		uintptr(unsafe.Pointer(ptr)), 0, sndFilename|sndAsync|sndNoDefault)
	if res == 0 {
		if callErr != nil && !errors.Is(callErr, windows.ERROR_SUCCESS) {
			return fmt.Errorf("PlaySoundW: %w", callErr)
		}
		return errors.New("PlaySoundW failed")
	}
	return nil
}
