package tui

import (
	"testing"

	"tuick-tack/internal/protocol"
)

func TestResizeSequence(t *testing.T) {
	if got, want := resizeSequence(140, 45), "\x1b[8;45;140t"; got != want {
		t.Fatalf("resizeSequence = %q, want %q", got, want)
	}
}

func TestHelloWindowAppliesOnce(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	var applied [][2]int
	m.applyWindow = func(w, h int) { applied = append(applied, [2]int{w, h}) }

	if cmd := m.noteHelloWindow(&protocol.WindowSizePayload{Width: 140, Height: 45}); cmd != nil {
		t.Fatal("connecting should not schedule a save")
	}
	if len(applied) != 1 || applied[0] != [2]int{140, 45} {
		t.Fatalf("applied = %v, want one 140x45 resize", applied)
	}
	if !m.windowSupported || !m.windowArmed || m.windowSavedW != 140 || m.windowSavedH != 45 {
		t.Fatalf("unexpected window state: supported=%v armed=%v saved=%dx%d",
			m.windowSupported, m.windowArmed, m.windowSavedW, m.windowSavedH)
	}

	// A reconnect must not reapply the stored size over a user resize.
	m.noteHelloWindow(&protocol.WindowSizePayload{Width: 140, Height: 45})
	if len(applied) != 1 {
		t.Fatalf("reconnect applied the size again: %v", applied)
	}
}

func TestHelloBeforeStartupDefersApply(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = false
	var applied [][2]int
	m.applyWindow = func(w, h int) { applied = append(applied, [2]int{w, h}) }

	m.noteHelloWindow(&protocol.WindowSizePayload{Width: 140, Height: 45})
	if len(applied) != 0 {
		t.Fatal("the stored size must wait for the terminal's startup size")
	}
	if !m.windowApplyPending || m.windowArmed {
		t.Fatalf("pending=%v armed=%v, want pending and disarmed",
			m.windowApplyPending, m.windowArmed)
	}

	if cmd := m.handleWindowSize(120, 40); cmd != nil {
		t.Fatal("the startup size must not schedule a save")
	}
	if len(applied) != 1 || applied[0] != [2]int{140, 45} {
		t.Fatalf("applied = %v, want one 140x45 resize", applied)
	}
	if m.windowApplyPending || !m.windowArmed || m.windowDirty {
		t.Fatalf("pending=%v armed=%v dirty=%v",
			m.windowApplyPending, m.windowArmed, m.windowDirty)
	}
}

func TestHelloWindowSkipsMatchingSize(t *testing.T) {
	m, _ := testModel(140, 45)
	m.sizeKnown = true
	applied := false
	m.applyWindow = func(int, int) { applied = true }

	m.noteHelloWindow(&protocol.WindowSizePayload{Width: 140, Height: 45})
	if applied {
		t.Fatal("the stored size matches the current one and should not be applied")
	}
	if !m.windowArmed {
		t.Fatal("window should be armed after the handshake")
	}
}

func TestStartupWindowSizeIsNotSaved(t *testing.T) {
	m, _ := testModel(80, 24)
	m.windowSupported = true
	m.windowConnected = true

	if cmd := m.handleWindowSize(120, 40); cmd != nil {
		t.Fatal("the first size after the handshake must not be saved")
	}
	if !m.windowArmed || m.windowDirty {
		t.Fatalf("armed=%v dirty=%v, want armed and clean", m.windowArmed, m.windowDirty)
	}
}

func TestResizeSchedulesAndSaves(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowConnected = true
	m.windowArmed = true
	m.windowSavedW, m.windowSavedH = 80, 24

	var sent [2]int
	m.sendWindow = func(w, h int) error {
		sent = [2]int{w, h}
		return nil
	}

	cmd := m.handleWindowSize(100, 30)
	if cmd == nil {
		t.Fatal("a user resize should schedule a save")
	}
	if !m.windowDirty {
		t.Fatal("a user resize should mark the size dirty")
	}

	// A newer resize makes the older debounce tick a no-op.
	if cmd := m.handleWindowSave(windowSaveMsg{width: 90, height: 28}); cmd != nil {
		t.Fatal("a superseded size must not be saved")
	}
	if sent != [2]int{} {
		t.Fatalf("stale size was sent: %v", sent)
	}

	cmd = m.handleWindowSave(windowSaveMsg{width: 100, height: 30})
	if cmd == nil {
		t.Fatal("the current size should be sent after the debounce")
	}
	msg, ok := cmd().(windowSavedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("unexpected save result: %#v", msg)
	}
	if sent != [2]int{100, 30} {
		t.Fatalf("sent = %v, want 100x30", sent)
	}

	m.handleWindowSaved(msg)
	if m.windowSavedW != 100 || m.windowSavedH != 30 || m.windowDirty {
		t.Fatalf("unexpected saved state: %dx%d dirty=%v",
			m.windowSavedW, m.windowSavedH, m.windowDirty)
	}
}

func TestOfflineResizeIsSavedAfterReconnect(t *testing.T) {
	m, _ := testModel(80, 24)
	m.connected = false
	m.sizeKnown = false

	m.handleWindowSize(80, 24) // startup size
	m.handleWindowSize(90, 28) // user resize while connecting
	if !m.windowChangedOffline {
		t.Fatal("resize before the handshake was not recorded")
	}

	m.connected = true
	applied := false
	m.applyWindow = func(int, int) { applied = true }
	cmd := m.noteHelloWindow(&protocol.WindowSizePayload{Width: 140, Height: 45})
	if applied {
		t.Fatal("the stored size must not override a resize made while connecting")
	}
	if cmd == nil || !m.windowDirty {
		t.Fatal("the offline size should be queued after the handshake")
	}
}

func TestNoWindowSupportDisablesSaving(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.noteHelloWindow(nil)
	if m.windowSupported {
		t.Fatal("a hello without window must disable the preference")
	}
	if cmd := m.handleWindowSize(100, 30); cmd != nil {
		t.Fatal("an older daemon must not receive window updates")
	}
}

func TestFlushWindowSizeOnClose(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowDirty = true
	m.width, m.height = 110, 35
	m.windowSavedW, m.windowSavedH = 80, 24
	var sent [2]int
	m.sendWindow = func(w, h int) error {
		sent = [2]int{w, h}
		return nil
	}

	m.flushWindowSize()
	if sent != [2]int{110, 35} {
		t.Fatalf("sent = %v, want 110x35", sent)
	}
	if m.windowDirty || m.windowSavedW != 110 || m.windowSavedH != 35 {
		t.Fatalf("unexpected state after flush: dirty=%v saved=%dx%d",
			m.windowDirty, m.windowSavedW, m.windowSavedH)
	}
}

func TestFlushWindowSizeWithoutChanges(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowSavedW, m.windowSavedH = 80, 24
	sent := false
	m.sendWindow = func(int, int) error {
		sent = true
		return nil
	}

	m.flushWindowSize()
	if sent {
		t.Fatal("an unchanged size must not be sent")
	}
}
