//go:build darwin

package daemon

import (
	"io"
	"log"
	"testing"

	"tuick-tack/internal/protocol"
	"tuick-tack/internal/update"
)

func testUpdateInfo() update.Info {
	return update.Info{
		Version:      "v9.9.9",
		URL:          "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9",
		MacOSURL:     "https://github.com/Juankyyy/tuick-tack/releases/download/v9.9.9/tuick-tack-macos-universal.zip",
		MacOSDigest:  testDigest,
		MacOSSumsURL: "https://github.com/Juankyyy/tuick-tack/releases/download/v9.9.9/tuick-tack-macos-universal.zip.sha256",
	}
}

func TestUpdatePayloadCapabilities(t *testing.T) {
	d := &Daemon{logger: log.New(io.Discard, "", 0)}
	info := testUpdateInfo()
	d.setUpdate(info)
	p := d.Update()
	if p == nil || p.Setup || !p.Portable {
		t.Fatalf("payload = %+v, want only the macOS download", p)
	}

	info.MacOSURL = ""
	d.setUpdate(info)
	p = d.Update()
	if p == nil || p.Setup || p.Portable {
		t.Fatalf("payload without the zip = %+v", p)
	}
}

func TestInstallUpdateUnsupported(t *testing.T) {
	var events []protocol.UpdateProgressMessage
	d := &Daemon{
		logger:   log.New(io.Discard, "", 0),
		progress: func(m protocol.UpdateProgressMessage) { events = append(events, m) },
	}
	d.setUpdate(testUpdateInfo())
	d.installUpdate(nil, true)
	if len(events) != 1 || events[0].Error != "unsupported" || events[0].Kind != "setup" {
		t.Fatalf("macOS install: %+v", events)
	}
}
