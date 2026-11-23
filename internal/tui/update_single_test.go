package tui

import (
	"strings"
	"testing"

	"tuick-tack/internal/install"
	"tuick-tack/internal/protocol"
)

// A single-update platform (macOS) must not offer the Windows Setup/Portable
// switch and must describe the .app zip. The flag is injected so this runs on
// every host, not only on macOS.
func TestUpdateDialogSingleAction(t *testing.T) {
	for _, size := range [][2]int{{48, 22}, {80, 24}, {120, 40}} {
		small, _ := testModel(size[0], size[1])
		small.setUpdate(&protocol.UpdatePayload{
			Latest: "v9.9.9", URL: "https://example.test/release", Portable: true,
		})
		small.installInfo = install.Info{Installed: true, Dir: "/Applications/Tuick-Tack.app"}
		small.singleUpdate = true
		small.openUpdateModal()
		frame := assertFrame(t, small, size[0], size[1])
		if !strings.Contains(frame, "download for macOS") {
			t.Errorf("%dx%d single-update chooser has no macOS download action:\n%s", size[0], size[1], frame)
		}
	}

	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{
		Latest: "v9.9.9", URL: "https://example.test/release", Portable: true,
	})
	m.installInfo = install.Info{Installed: true, Dir: "/Applications/Tuick-Tack.app"}
	m.singleUpdate = true
	m.openUpdateModal()

	if m.updateType != updateTypePortable || m.updateFocus != hitUpdatePortable {
		t.Fatalf("single update should start on the download, type=%q focus=%v", m.updateType, m.updateFocus)
	}
	if m.bothUpdateTypes() {
		t.Fatal("a single update must not offer a second copy type")
	}
	if rows := m.updateTypeRows(60); rows != 0 {
		t.Fatalf("a single update should not reserve type chips, rows=%d", rows)
	}

	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{
		"Downloads the .zip; replace Tuick-Tack.app.",
		"download", "github", "later",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("single-update chooser missing %q:\n%s", want, plain)
		}
	}
	for _, unwanted := range []string{"Setup", "[Tab]", "Portable · tuick-tack.exe"} {
		if strings.Contains(plain, unwanted) {
			t.Errorf("single-update chooser should not show %q:\n%s", unwanted, plain)
		}
	}

	m.updateStep = updateStepProgress
	m.updateKind = "portable"
	progress := assertFrame(t, m, 80, 24)
	if !strings.Contains(progress, "macOS update") {
		t.Fatalf("single-update progress title missing:\n%s", progress)
	}

	path := "/Users/a/Downloads/tuick-tack-v9.9.9-macos-universal.zip"
	m.handleUpdateProgress(protocol.NewUpdateProgress(protocol.UpdateProgressMessage{
		Kind: "portable", Phase: "done", Received: 100, Total: 100, Path: path,
	}))
	if m.updateStep != updateStepPortableDone {
		t.Fatalf("step = %v, want the download result", m.updateStep)
	}
	done := assertFrame(t, m, 80, 24)
	for _, want := range []string{"macOS update downloaded", "Unzip it and replace the current Tuick-Tack.app."} {
		if !strings.Contains(done, want) {
			t.Errorf("single-update result missing %q:\n%s", want, done)
		}
	}
}
