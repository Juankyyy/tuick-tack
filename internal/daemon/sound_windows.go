package daemon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"tuick-tack/assets"
	"tuick-tack/internal/config"
)

const (
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
	sndFilename  = 0x00020000
)

var playSoundProc = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

// soundAssets maps the stable config IDs to the embedded WAVs. Keeping the map
// here makes a missing asset a compile-time-adjacent test failure instead of a
// silent "no sound" at runtime.
var soundAssets = map[string][]byte{
	config.SoundChime:   assets.ChimeWAV,
	config.SoundMarimba: assets.MarimbaWAV,
	config.SoundDigital: assets.DigitalWAV,
	config.SoundDeep:    assets.DeepWAV,
}

// soundPlayer extracts the selected sound at the configured volume into the
// cache and plays it through PlaySoundW. Failures are non-fatal: the countdown
// still finishes and the tray alerts visually.
type soundPlayer struct {
	mu       sync.Mutex
	cacheDir string
	name     string
	volume   int
	path     string
}

// newSoundPlayer returns a player rooted at %LOCALAPPDATA%\tuick-tack. The
// sounds subdirectory is created lazily by Prepare. A zero player is usable and
// just returns an error instead of playing.
func newSoundPlayer(cacheDir string) (*soundPlayer, error) {
	if cacheDir == "" {
		return &soundPlayer{}, errors.New("no cache directory")
	}
	// Remove the pre-v2 chime so upgrades do not leave a stale 350 KB file
	// behind. The file lives in our own per-user cache directory.
	_ = os.Remove(filepath.Join(cacheDir, "chime.wav"))
	return &soundPlayer{cacheDir: filepath.Join(cacheDir, "sounds")}, nil
}

// Prepare makes sure the scaled WAV for name and volume is in the cache. A
// volume of zero removes the active path; Play then only stops any sound.
func (p *soundPlayer) Prepare(name string, volume int) error {
	if p == nil {
		return errors.New("sound is not available")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.prepareLocked(name, volume)
}

// Play starts the selected sound asynchronously. PlaySoundW returns
// immediately. It prepares the file on demand if the config changed.
func (p *soundPlayer) Play(name string, volume int) error {
	if p == nil {
		return errors.New("sound is not available")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.prepareLocked(name, volume); err != nil {
		return err
	}
	if p.path == "" {
		p.stopLocked()
		return nil
	}
	return playFile(p.path)
}

// Stop silences a sound that is still playing.
func (p *soundPlayer) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *soundPlayer) stopLocked() {
	_, _, _ = playSoundProc.Call(0, 0, 0)
}

func (p *soundPlayer) prepareLocked(name string, volume int) error {
	if p.cacheDir == "" {
		return errors.New("sound cache is not available")
	}
	if !config.ValidSound(name) {
		return fmt.Errorf("unknown sound %q", name)
	}
	if !config.ValidVolume(volume) {
		return fmt.Errorf("volume %d is outside %d..%d", volume, config.MinVolume, config.MaxVolume)
	}
	if volume == 0 {
		p.name = name
		p.volume = 0
		p.path = ""
		p.cleanupLocked("")
		return nil
	}
	if p.name == name && p.volume == volume && p.path != "" {
		if _, err := os.Stat(p.path); err == nil {
			return nil
		}
	}

	data, ok := soundAssets[name]
	if !ok {
		return fmt.Errorf("sound %q has no asset", name)
	}
	scaled, err := scaleWAV(data, volume)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.cacheDir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(p.cacheDir, fmt.Sprintf("%s-%d.wav", name, volume))
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, scaled) {
		if err := writeAtomic(path, scaled); err != nil {
			return err
		}
	}
	p.name = name
	p.volume = volume
	p.path = path
	p.cleanupLocked(path)
	return nil
}

func (p *soundPlayer) cleanupLocked(keep string) {
	entries, err := os.ReadDir(p.cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wav") {
			continue
		}
		path := filepath.Join(p.cacheDir, entry.Name())
		if path == keep {
			continue
		}
		_ = os.Remove(path)
	}
}

// writeAtomic writes data to a temporary file in the same directory and renames
// it over path. The temporary file is removed if any step fails.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sound-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// scaleWAV returns a copy of a PCM WAV with every sample multiplied by
// volume/100. The header and chunk layout are preserved. Only 16-bit PCM is
// accepted, which is the format of the generated pack.
func scaleWAV(data []byte, volume int) ([]byte, error) {
	if !config.ValidVolume(volume) {
		return nil, fmt.Errorf("volume %d is outside %d..%d", volume, config.MinVolume, config.MaxVolume)
	}
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("sound is not a RIFF/WAVE file")
	}

	var (
		haveFmt  bool
		haveData bool
		format   uint16
		bits     uint16
		dataPos  int
		dataLen  int
	)
	for pos := 12; pos+8 <= len(data); {
		id := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		body := pos + 8
		if size < 0 || body > len(data) || size > len(data)-body {
			return nil, fmt.Errorf("truncated %q chunk", id)
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, errors.New("fmt chunk is too short")
			}
			format = binary.LittleEndian.Uint16(data[body : body+2])
			bits = binary.LittleEndian.Uint16(data[body+14 : body+16])
			haveFmt = true
		case "data":
			dataPos = body
			dataLen = size
			haveData = true
		}
		pos = body + size
		if size%2 == 1 {
			pos++
		}
	}
	if !haveFmt {
		return nil, errors.New("sound has no fmt chunk")
	}
	if !haveData {
		return nil, errors.New("sound has no data chunk")
	}
	if format != 1 {
		return nil, fmt.Errorf("unsupported WAV format %d (need PCM)", format)
	}
	if bits != 16 {
		return nil, fmt.Errorf("unsupported WAV bit depth %d (need 16)", bits)
	}
	if dataLen%2 != 0 {
		return nil, errors.New("WAV data length is not a whole number of samples")
	}

	out := append([]byte(nil), data...)
	if volume == 100 {
		return out, nil
	}
	gain := float64(volume) / 100
	for i := dataPos; i+1 < dataPos+dataLen; i += 2 {
		sample := int16(binary.LittleEndian.Uint16(out[i : i+2]))
		value := int(math.Round(float64(sample) * gain))
		if value > 32767 {
			value = 32767
		}
		if value < -32768 {
			value = -32768
		}
		binary.LittleEndian.PutUint16(out[i:i+2], uint16(int16(value)))
	}
	return out, nil
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
