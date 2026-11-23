package daemon

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
	sndFilename  = 0x00020000
)

var playSoundProc = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

// playFile starts path asynchronously. PlaySoundW returns immediately and the
// sound plays at the volume already baked into the WAV.
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

// stopPlayback silences a sound that is still playing. A zero handle stops the
// current PlaySoundW request.
func stopPlayback() {
	_, _, _ = playSoundProc.Call(0, 0, 0)
}
