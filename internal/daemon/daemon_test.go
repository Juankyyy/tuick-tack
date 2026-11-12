package daemon

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"tuick-tack/internal/config"
)

func TestSameExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "tuick-tack-tray.exe")
	if !sameExecutable(`"`+exe+`"`, exe) {
		t.Fatal("quoted path should match")
	}
	if !sameExecutable(exe, exe) {
		t.Fatal("plain path should match")
	}
	if sameExecutable(`"`+filepath.Join(dir, "other.exe")+`"`, exe) {
		t.Fatal("different path should not match")
	}
	if sameExecutable("", exe) {
		t.Fatal("empty value should not match")
	}
}

func TestInstanceMutexName(t *testing.T) {
	got := InstanceMutexName(`\\.\pipe\tuick-tack-juanky-1`)
	if got != "tuick-tack-juanky-1-mutex" {
		t.Fatalf("mutex name = %q", got)
	}
}

func TestTUIInstanceMutexName(t *testing.T) {
	got := TUIInstanceMutexName(`\\.\pipe\tuick-tack-juanky-1`)
	if got != "tuick-tack-juanky-1-tui-mutex" {
		t.Fatalf("TUI mutex name = %q", got)
	}
}

func TestTUIRunning(t *testing.T) {
	pipe := `\\.\pipe\tuick-tack-test-` + filepath.Base(t.TempDir())
	if TUIRunning(pipe) {
		t.Fatal("reported running before acquire")
	}

	release, already, err := AcquireTUIInstance(pipe)
	if err != nil {
		t.Fatalf("AcquireTUIInstance: %v", err)
	}
	if already {
		t.Fatal("first acquire reported already running")
	}
	if !TUIRunning(pipe) {
		t.Fatal("reported not running after acquire")
	}

	second, already, err := AcquireTUIInstance(pipe)
	if err != nil {
		t.Fatalf("second AcquireTUIInstance: %v", err)
	}
	if !already {
		t.Fatal("second acquire should report already running")
	}
	second()

	release()
	if TUIRunning(pipe) {
		t.Fatal("reported running after release")
	}
}

func TestAcquireInstance(t *testing.T) {
	name := "tuick-tack-test-mutex-" + filepath.Base(t.TempDir())
	first, already, err := AcquireInstance(name)
	if err != nil {
		t.Fatalf("first AcquireInstance: %v", err)
	}
	if already {
		t.Fatal("first acquire reported already running")
	}
	defer windows.CloseHandle(first)

	second, already, err := AcquireInstance(name)
	if err != nil {
		t.Fatalf("second AcquireInstance: %v", err)
	}
	if !already {
		t.Fatal("second acquire should report already running")
	}
	if second != 0 {
		t.Fatalf("second handle = %v, want 0", second)
	}
}

func TestSoundCatalogMatchesConfig(t *testing.T) {
	if len(soundAssets) != len(config.Sounds) {
		t.Fatalf("sound assets = %d, config sounds = %d", len(soundAssets), len(config.Sounds))
	}
	for _, name := range config.Sounds {
		data, ok := soundAssets[name]
		if !ok || len(data) < 1000 {
			t.Errorf("sound %q has no usable asset", name)
		}
	}
}

func TestNewSoundPlayerPreparesScaledCache(t *testing.T) {
	dir := t.TempDir()
	p, err := newSoundPlayer(dir)
	if err != nil {
		t.Fatalf("newSoundPlayer: %v", err)
	}
	if err := p.Prepare(config.SoundDigital, 80); err != nil {
		t.Fatalf("Prepare digital: %v", err)
	}
	if p.path == "" {
		t.Fatal("Prepare left an empty path")
	}
	if want := filepath.Join(dir, "sounds", "digital-80.wav"); p.path != want {
		t.Fatalf("path = %q, want %q", p.path, want)
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		t.Fatalf("read prepared sound: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("prepared sound is suspiciously small: %d", len(data))
	}
	original, err := scaleWAV(soundAssets[config.SoundDigital], 80)
	if err != nil {
		t.Fatalf("scaleWAV: %v", err)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("prepared cache does not match the scaled asset")
	}

	if err := p.Prepare(config.SoundMarimba, 50); err != nil {
		t.Fatalf("Prepare marimba: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "sounds"))
	if err != nil {
		t.Fatal(err)
	}
	wavs := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".wav" {
			wavs++
		}
	}
	if wavs != 1 {
		t.Fatalf("stale sound files were not cleaned up: %d WAVs", wavs)
	}

	if err := p.Prepare(config.SoundDeep, 0); err != nil {
		t.Fatalf("Prepare volume 0: %v", err)
	}
	if p.path != "" {
		t.Fatalf("volume 0 should have no path, got %q", p.path)
	}
}

func TestScaleWAV(t *testing.T) {
	original := testWAV([]int16{10000, -10000, 32767, -32768})

	full, err := scaleWAV(original, 100)
	if err != nil {
		t.Fatalf("scale 100: %v", err)
	}
	if !bytes.Equal(full, original) {
		t.Fatal("volume 100 must preserve the original bytes")
	}

	half, err := scaleWAV(original, 50)
	if err != nil {
		t.Fatalf("scale 50: %v", err)
	}
	got := wavSamples(half)
	want := []int16{5000, -5000, 16384, -16384}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d = %d, want %d", i, got[i], want[i])
		}
	}

	zero, err := scaleWAV(original, 0)
	if err != nil {
		t.Fatalf("scale 0: %v", err)
	}
	for i, s := range wavSamples(zero) {
		if s != 0 {
			t.Fatalf("volume 0 sample %d = %d", i, s)
		}
	}

	if _, err := scaleWAV(original, 101); err == nil {
		t.Fatal("volume 101 should fail")
	}
	if _, err := scaleWAV([]byte("not a wav"), 50); err == nil {
		t.Fatal("non-WAV data should fail")
	}
	if _, err := scaleWAV(original[:20], 50); err == nil {
		t.Fatal("truncated WAV should fail")
	}
}

func TestScaleWAVAssets(t *testing.T) {
	for _, name := range config.Sounds {
		original := soundAssets[name]
		for _, volume := range []int{0, 5, 50, 80, 100} {
			scaled, err := scaleWAV(original, volume)
			if err != nil {
				t.Fatalf("scale %q at %d%%: %v", name, volume, err)
			}
			if len(scaled) != len(original) {
				t.Fatalf("scale %q at %d%% changed size: %d -> %d",
					name, volume, len(original), len(scaled))
			}
			if volume == 100 && !bytes.Equal(scaled, original) {
				t.Fatalf("scale %q at 100%% changed bytes", name)
			}
		}
	}
}

func testWAV(samples []int16) []byte {
	dataSize := len(samples) * 2
	buf := make([]byte, 44+dataSize)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataSize))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1)
	binary.LittleEndian.PutUint16(buf[22:], 1)
	binary.LittleEndian.PutUint32(buf[24:], 44100)
	binary.LittleEndian.PutUint32(buf[28:], 44100*2)
	binary.LittleEndian.PutUint16(buf[32:], 2)
	binary.LittleEndian.PutUint16(buf[34:], 16)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataSize))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[44+i*2:], uint16(s))
	}
	return buf
}

func wavSamples(data []byte) []int16 {
	out := make([]int16, (len(data)-44)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[44+i*2:]))
	}
	return out
}

func TestLaunchHelpersDoNotPanic(t *testing.T) {
	if got := InstanceMutexName(`\\.\pipe\x`); got != "x-mutex" {
		t.Fatalf("mutex name = %q", got)
	}
	if fileExists(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing file reported as existing")
	}
}
