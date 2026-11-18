package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/update"
)

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func testUpdateInfo() update.Info {
	return update.Info{
		Version:        "v9.9.9",
		URL:            "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9",
		SetupURL:       "https://github.com/Juankyyy/tuick-tack/releases/download/v9.9.9/tuick-tack-setup.exe",
		SetupDigest:    testDigest,
		PortableURL:    "https://github.com/Juankyyy/tuick-tack/releases/download/v9.9.9/tuick-tack.exe",
		PortableDigest: testDigest,
	}
}

func TestUpdatePayloadCapabilities(t *testing.T) {
	d := &Daemon{logger: log.New(io.Discard, "", 0)}
	info := testUpdateInfo()
	d.setUpdate(info)
	p := d.Update()
	if p == nil || !p.Setup || !p.Portable {
		t.Fatalf("payload = %+v, want setup and portable", p)
	}

	info.SetupURL = ""
	info.PortableURL = ""
	d.setUpdate(info)
	p = d.Update()
	if p == nil || p.Setup || p.Portable {
		t.Fatalf("payload without assets = %+v", p)
	}
}

func TestInstallUpdateGuards(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	defer func() { buildinfo.Version = oldVersion }()

	newDaemon := func() (*Daemon, *[]protocol.UpdateProgressMessage) {
		var events []protocol.UpdateProgressMessage
		return &Daemon{
			logger:   log.New(io.Discard, "", 0),
			engine:   engine.New(engine.SystemClock()),
			progress: func(m protocol.UpdateProgressMessage) { events = append(events, m) },
		}, &events
	}

	d, events := newDaemon()
	d.installUpdate(nil, false)
	if len(*events) != 1 || (*events)[0].Error != "unavailable" {
		t.Fatalf("without a release: %+v", *events)
	}

	d, events = newDaemon()
	d.setUpdate(testUpdateInfo())
	d.engine.Start(5*time.Minute, "Focus", "")
	d.installUpdate(nil, false)
	if len(*events) != 1 || (*events)[0].Error != "timer_active" || (*events)[0].Kind != "setup" {
		t.Fatalf("running timer: %+v", *events)
	}
	if d.updating {
		t.Fatal("a rejected install must not leave the updating flag set")
	}

	d, events = newDaemon()
	d.setUpdate(testUpdateInfo())
	d.updating = true
	d.installUpdate(nil, true)
	if len(*events) != 1 || (*events)[0].Error != "busy" {
		t.Fatalf("busy state: %+v", *events)
	}

	buildinfo.Version = "dev"
	d, events = newDaemon()
	d.setUpdate(testUpdateInfo())
	d.installUpdate(nil, true)
	if len(*events) != 1 || (*events)[0].Error != "unsupported" {
		t.Fatalf("dev build: %+v", *events)
	}
}

func TestInstallUpdateDownloadsAndSpawns(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	defer func() { buildinfo.Version = oldVersion }()

	content := []byte("installer payload")
	sum := sha256.Sum256(content)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	var events []protocol.UpdateProgressMessage
	spawned := ""
	d := &Daemon{
		logger:     log.New(io.Discard, "", 0),
		engine:     engine.New(engine.SystemClock()),
		pipe:       `\\.\pipe\tuick-tack-test-update`,
		selfExe:    `C:\Apps\tuick-tack.exe`,
		downloader: &update.Downloader{Client: srv.Client()},
		updateDir:  func() (string, error) { return t.TempDir(), nil },
		progress:   func(m protocol.UpdateProgressMessage) { events = append(events, m) },
		spawnUpdater: func(setup, fallback, pipe string) error {
			spawned = setup
			return nil
		},
		quitCh: make(chan struct{}),
	}
	info := testUpdateInfo()
	info.SetupURL = srv.URL + "/setup"
	info.SetupSize = int64(len(content))
	info.SetupDigest = "sha256:" + hex.EncodeToString(sum[:])
	info.ChecksumsURL = ""
	info.PortableURL = ""
	d.setUpdate(info)

	d.installUpdate(nil, true)
	if spawned == "" {
		t.Fatalf("helper was not started: %+v", events)
	}
	if filepath.Base(spawned) != "tuick-tack-setup.exe" {
		t.Fatalf("setup path = %q", spawned)
	}
	if _, err := os.Stat(spawned); err != nil {
		t.Fatalf("downloaded setup: %v", err)
	}
	if !d.updateQuit.Load() {
		t.Fatal("a successful install should request the update shutdown")
	}
	foundInstalling := false
	for _, ev := range events {
		if ev.Phase == updatePhaseInstalling {
			foundInstalling = true
		}
		if ev.Phase == updatePhaseFailed {
			t.Fatalf("unexpected failure event: %+v", ev)
		}
	}
	if !foundInstalling {
		t.Fatalf("events = %+v", events)
	}
}

func TestDownloadPortableGuards(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	defer func() { buildinfo.Version = oldVersion }()

	var events []protocol.UpdateProgressMessage
	d := &Daemon{
		logger:   log.New(io.Discard, "", 0),
		progress: func(m protocol.UpdateProgressMessage) { events = append(events, m) },
	}
	d.downloadPortable(nil)
	if len(events) != 1 || events[0].Error != "unavailable" {
		t.Fatalf("without a release: %+v", events)
	}

	events = nil
	d.setUpdate(testUpdateInfo())
	d.portableActive = true
	d.downloadPortable(nil)
	if len(events) != 1 || events[0].Error != "busy" {
		t.Fatalf("busy state: %+v", events)
	}
}

func TestSanitizeVersion(t *testing.T) {
	tests := map[string]string{
		"v0.3.0":      "v0.3.0",
		"v1.2.3-rc.1": "v1.2.3-rc.1",
		"../../evil":  "..-..-evil",
		"":            "update",
		"!!":          "update",
	}
	for in, want := range tests {
		if got := sanitizeVersion(in); got != want {
			t.Errorf("sanitizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDownloadCode(t *testing.T) {
	if got := downloadCode(update.ErrChecksum); got != "checksum" {
		t.Fatalf("checksum code = %q", got)
	}
	if got := downloadCode(io.EOF); got != "network" {
		t.Fatalf("network code = %q", got)
	}
}
