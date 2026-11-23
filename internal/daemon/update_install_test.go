package daemon

import (
	"io"
	"log"
	"testing"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/update"
)

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

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
