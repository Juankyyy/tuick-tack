package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/config"
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
	m.windowOwned = true
	var order []string
	var applied [][2]int
	m.applyWindow = func(w, h int) {
		order = append(order, "resize")
		applied = append(applied, [2]int{w, h})
	}
	var moved [][2]int
	m.moveWindow = func(x, y int) bool {
		order = append(order, "move")
		moved = append(moved, [2]int{x, y})
		return true
	}
	m.windowMaximizedFn = func() bool { return false }
	pos := &config.WindowPosition{X: 93, Y: 100}

	if cmd := m.noteHelloWindow(&protocol.WindowSizePayload{
		Width: 140, Height: 45, Position: pos,
	}); cmd != nil {
		t.Fatal("connecting should not schedule a save")
	}
	if len(applied) != 1 || applied[0] != [2]int{140, 45} {
		t.Fatalf("applied = %v, want one 140x45 resize", applied)
	}
	if len(moved) != 1 || moved[0] != [2]int{93, 100} {
		t.Fatalf("moved = %v, want one move to 93,100", moved)
	}
	if len(order) != 2 || order[0] != "move" || order[1] != "resize" {
		t.Fatalf("placement order = %v, want move before resize", order)
	}
	if !m.windowSupported || !m.windowArmed || m.windowSavedW != 140 || m.windowSavedH != 45 {
		t.Fatalf("unexpected window state: supported=%v armed=%v saved=%dx%d",
			m.windowSupported, m.windowArmed, m.windowSavedW, m.windowSavedH)
	}
	if !m.windowHasPos || !m.posKnown || !m.posArmed ||
		m.windowSavedX != 93 || m.windowSavedY != 100 {
		t.Fatalf("unexpected position state: has=%v known=%v armed=%v saved=%d,%d x=%d",
			m.windowHasPos, m.posKnown, m.posArmed, m.windowSavedX, m.windowSavedY, m.windowX)
	}

	// A reconnect must not reapply the stored placement over a user change.
	pos2 := &config.WindowPosition{X: 10, Y: 20}
	m.noteHelloWindow(&protocol.WindowSizePayload{Width: 140, Height: 45, Position: pos2})
	if len(applied) != 1 || len(moved) != 1 {
		t.Fatalf("reconnect reapplied the placement: applied=%v moved=%v", applied, moved)
	}
	if m.windowSavedX != 10 || m.windowSavedY != 20 {
		t.Fatalf("reconnect did not refresh the saved position: %d,%d",
			m.windowSavedX, m.windowSavedY)
	}
}

func TestHelloBeforeStartupDefersApply(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = false
	m.windowOwned = true
	var applied [][2]int
	m.applyWindow = func(w, h int) { applied = append(applied, [2]int{w, h}) }
	m.moveWindow = func(int, int) bool { return true }
	m.windowMaximizedFn = func() bool { return false }

	m.noteHelloWindow(&protocol.WindowSizePayload{
		Width: 140, Height: 45, Position: &config.WindowPosition{X: 93, Y: 100},
	})
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

func TestResizeAfterStoredPlacementClearsScreen(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowOwned = true
	m.applyWindow = func(int, int) {}
	m.moveWindow = func(int, int) bool { return true }
	m.windowMaximizedFn = func() bool { return false }

	m.noteHelloWindow(&protocol.WindowSizePayload{Width: 140, Height: 45})
	if !m.windowClearPending {
		t.Fatal("applying the stored size should arm a clear")
	}

	_, cmd := m.Update(tea.WindowSizeMsg{Width: 140, Height: 45})
	if cmd == nil {
		t.Fatal("the resize after the stored placement should force a clear")
	}
	if m.windowClearPending {
		t.Fatal("the clear should be consumed by the resize")
	}
}

func TestHelloWindowSkipsMatchingSize(t *testing.T) {
	m, _ := testModel(140, 45)
	m.sizeKnown = true
	m.windowOwned = true
	applied := false
	m.applyWindow = func(int, int) { applied = true }
	m.moveWindow = func(int, int) bool { return true }
	m.windowMaximizedFn = func() bool { return false }

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

	var sentSize [2]int
	var sentPos *config.WindowPosition
	m.sendWindow = func(w, h int, pos *config.WindowPosition) error {
		sentSize = [2]int{w, h}
		sentPos = pos
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
	if sentSize != [2]int{} {
		t.Fatalf("stale size was sent: %v", sentSize)
	}

	cmd = m.handleWindowSave(windowSaveMsg{width: 100, height: 30})
	if cmd == nil {
		t.Fatal("the current size should be sent after the debounce")
	}
	msg, ok := cmd().(windowSavedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("unexpected save result: %#v", msg)
	}
	if sentSize != [2]int{100, 30} || sentPos != nil {
		t.Fatalf("sent = %v pos=%v, want 100x30 without position", sentSize, sentPos)
	}

	m.handleWindowSaved(msg)
	if m.windowSavedW != 100 || m.windowSavedH != 30 || m.windowDirty {
		t.Fatalf("unexpected saved state: %dx%d dirty=%v",
			m.windowSavedW, m.windowSavedH, m.windowDirty)
	}
}

func TestPositionBaselineIsNotSaved(t *testing.T) {
	m, clock := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowConnected = true
	m.windowOwned = true
	m.windowSavedW, m.windowSavedH = 80, 24
	x, y := 93, 100
	m.windowPositionFn = func() (int, int, bool) { return x, y, true }
	m.windowMaximizedFn = func() bool { return false }
	cmd := m.sampleWindow()
	if cmd != nil {
		t.Fatal("the first observed position is a baseline, not a move")
	}
	if !m.posArmed || m.windowX != 93 || m.windowY != 100 || m.windowHasPos {
		t.Fatalf("unexpected baseline: armed=%v pos=%d,%d has=%v",
			m.posArmed, m.windowX, m.windowY, m.windowHasPos)
	}

	clock.Advance(windowPosSampleInterval + time.Millisecond)
	x, y = 200, 300
	cmd = m.sampleWindow()
	if cmd == nil {
		t.Fatal("a real move should schedule a save")
	}
	if !m.posDirty || !m.windowHasPos || m.windowX != 200 || m.windowY != 300 {
		t.Fatalf("unexpected move state: dirty=%v has=%v pos=%d,%d",
			m.posDirty, m.windowHasPos, m.windowX, m.windowY)
	}

	var sentSize [2]int
	var sentPos *config.WindowPosition
	m.sendWindow = func(w, h int, pos *config.WindowPosition) error {
		sentSize = [2]int{w, h}
		sentPos = pos
		return nil
	}
	cmd = m.handleWindowSave(windowSaveMsg{width: 80, height: 24, x: 200, y: 300, hasPos: true})
	if cmd == nil {
		t.Fatal("the move should be sent after the debounce")
	}
	msg := cmd().(windowSavedMsg)
	if sentSize != [2]int{80, 24} || sentPos == nil ||
		sentPos.X != 200 || sentPos.Y != 300 {
		t.Fatalf("sent = %v pos=%v, want the moved position", sentSize, sentPos)
	}
	m.handleWindowSaved(msg)
	if m.posDirty || m.windowSavedX != 200 || m.windowSavedY != 300 {
		t.Fatalf("position not marked saved: dirty=%v saved=%d,%d",
			m.posDirty, m.windowSavedX, m.windowSavedY)
	}
}

func TestPositionSettleReappliesHostMove(t *testing.T) {
	m, clock := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowConnected = true
	m.windowOwned = true
	m.windowSavedW, m.windowSavedH = 80, 24
	m.windowMaximizedFn = func() bool { return false }
	hostX, hostY := 93, 100
	m.windowPositionFn = func() (int, int, bool) { return hostX, hostY, true }
	moves := 0
	m.moveWindow = func(x, y int) bool {
		moves++
		hostX, hostY = x, y
		return true
	}

	m.noteHelloPosition(&config.WindowPosition{X: 93, Y: 100}, true)
	if moves != 1 {
		t.Fatalf("initial placement moves = %d, want 1", moves)
	}
	if m.posSettleUntil.IsZero() {
		t.Fatal("a successful placement should arm the settle window")
	}

	// Windows Terminal can finish the handoff by cascading the fresh window.
	hostX, hostY = 200, 208
	clock.Advance(windowPosSampleInterval + time.Millisecond)
	if cmd := m.sampleWindow(); cmd != nil {
		t.Fatal("a host move during the settle must not be saved")
	}
	if moves != 2 || hostX != 93 || hostY != 100 {
		t.Fatalf("settle did not reapply: moves=%d host=%d,%d", moves, hostX, hostY)
	}
	if m.posDirty {
		t.Fatal("the settle marked the position dirty")
	}

	// Once settled, a real user move is still recorded.
	clock.Advance(windowPosSettle + windowPosSampleInterval)
	hostX, hostY = 300, 400
	if cmd := m.sampleWindow(); cmd == nil {
		t.Fatal("a move after the settle should schedule a save")
	}
	if !m.posDirty || m.windowX != 300 || m.windowY != 400 {
		t.Fatalf("unexpected move state: dirty=%v pos=%d,%d", m.posDirty, m.windowX, m.windowY)
	}
}

func TestFlushDuringPositionSettleKeepsStoredPosition(t *testing.T) {
	m, clock := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowConnected = true
	m.windowOwned = true
	m.windowSavedW, m.windowSavedH = 80, 24
	m.windowSavedX, m.windowSavedY = 93, 100
	m.windowHasPos = true
	m.posKnown = true
	m.posValid = true
	m.posArmed = true
	m.windowX, m.windowY = 93, 100
	m.posSettleUntil = clock.Now().Add(time.Second)
	m.windowPositionFn = func() (int, int, bool) { return 200, 208, true }
	m.windowDirty = true
	m.width, m.height = 90, 28

	var sentSize [2]int
	var sentPos *config.WindowPosition
	m.sendWindow = func(w, h int, pos *config.WindowPosition) error {
		sentSize = [2]int{w, h}
		sentPos = pos
		return nil
	}
	m.flushWindow()
	if sentPos != nil {
		t.Fatalf("closing during the settle saved the host cascade: %+v", sentPos)
	}
	if sentSize != [2]int{90, 28} {
		t.Fatalf("flushed size = %v, want 90x28", sentSize)
	}
	if m.windowSavedW != 90 || m.windowSavedH != 28 {
		t.Fatalf("saved size = %dx%d, want 90x28", m.windowSavedW, m.windowSavedH)
	}
	if m.windowSavedX != 93 || m.windowSavedY != 100 {
		t.Fatalf("stored position changed to %d,%d", m.windowSavedX, m.windowSavedY)
	}
}

func TestUnavailablePositionKeepsStoredValue(t *testing.T) {
	m, clock := testModel(80, 24)
	m.sizeKnown = true
	m.windowOwned = true
	m.moveWindow = func(int, int) bool { return false }
	m.windowMaximizedFn = func() bool { return false }
	m.windowPositionFn = func() (int, int, bool) { return 50, 60, true }
	m.noteHelloWindow(&protocol.WindowSizePayload{
		Width: 80, Height: 24, Position: &config.WindowPosition{X: 93, Y: 100},
	})
	if !m.posApplyPending {
		t.Fatal("a failed start should retry the stored position")
	}

	clock.Advance(windowPosApplyWindow + time.Second)
	m.sampleWindow()
	if m.posApplyPending || m.windowHasPos || m.posKnown {
		t.Fatalf("expired placement was not forgotten: pending=%v has=%v known=%v",
			m.posApplyPending, m.windowHasPos, m.posKnown)
	}
	if !m.posArmed || m.windowX != 50 || m.windowY != 60 {
		t.Fatalf("baseline not set: armed=%v pos=%d,%d",
			m.posArmed, m.windowX, m.windowY)
	}

	var sentPos *config.WindowPosition
	m.sendWindow = func(w, h int, pos *config.WindowPosition) error {
		sentPos = pos
		return nil
	}
	m.windowDirty = true
	m.width, m.height = 90, 28
	m.flushWindow()
	if sentPos != nil {
		t.Fatalf("size save overwrote the stored position: %+v", sentPos)
	}
}

func TestStalePositionSaveIsDropped(t *testing.T) {
	m, clock := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowConnected = true
	m.windowOwned = true
	m.windowSavedW, m.windowSavedH = 80, 24
	x, y := 100, 100
	m.windowPositionFn = func() (int, int, bool) { return x, y, true }
	m.windowMaximizedFn = func() bool { return false }
	m.sampleWindow() // baseline
	clock.Advance(windowPosSampleInterval + time.Millisecond)
	x, y = 200, 200
	if cmd := m.sampleWindow(); cmd == nil {
		t.Fatal("move not detected")
	}
	clock.Advance(windowPosSampleInterval + time.Millisecond)
	x, y = 300, 300
	m.sampleWindow()
	if cmd := m.handleWindowSave(windowSaveMsg{
		width: 80, height: 24, x: 200, y: 200, hasPos: true,
	}); cmd != nil {
		t.Fatal("a superseded position must not be saved")
	}
}

func TestFlushWindowSavesPosition(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.connected = true
	m.windowOwned = true
	m.windowSavedW, m.windowSavedH = 80, 24
	m.windowSavedX, m.windowSavedY = 93, 100
	m.windowX, m.windowY = 93, 100
	m.windowHasPos = true
	m.posKnown = true
	m.posArmed = true
	m.windowPositionFn = func() (int, int, bool) { return 110, 120, true }
	m.windowMaximizedFn = func() bool { return false }
	var sentPos *config.WindowPosition
	m.sendWindow = func(w, h int, pos *config.WindowPosition) error {
		sentPos = pos
		return nil
	}

	m.flushWindow()
	if sentPos == nil || sentPos.X != 110 || sentPos.Y != 120 {
		t.Fatalf("position not flushed: %+v", sentPos)
	}
	if m.posDirty || m.windowSavedX != 110 || m.windowSavedY != 120 {
		t.Fatalf("unexpected state after flush: dirty=%v saved=%d,%d",
			m.posDirty, m.windowSavedX, m.windowSavedY)
	}
}

func TestSharedConsoleSkipsPosition(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowOwned = false
	m.moveWindow = func(int, int) bool {
		t.Fatal("a shared console must not be moved")
		return false
	}
	m.windowPositionFn = func() (int, int, bool) {
		t.Fatal("a shared console must not be sampled")
		return 0, 0, false
	}
	m.windowMaximizedFn = func() bool { return false }

	m.noteHelloWindow(&protocol.WindowSizePayload{
		Width: 80, Height: 24, Position: &config.WindowPosition{X: 93, Y: 100},
	})
	if cmd := m.sampleWindow(); cmd != nil {
		t.Fatal("a shared console must not schedule a position save")
	}
	if m.windowHasPos || m.posArmed {
		t.Fatalf("shared console recorded a position: has=%v armed=%v",
			m.windowHasPos, m.posArmed)
	}
}

func TestMaximizedSizeIsNotSaved(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowConnected = true
	m.windowArmed = true
	m.windowOwned = true
	m.windowSavedW, m.windowSavedH = 80, 24
	m.windowMaximizedFn = func() bool { return true }

	if cmd := m.handleWindowSize(200, 50); cmd != nil {
		t.Fatal("the maximized grid must not be saved")
	}
	if m.windowDirty {
		t.Fatal("the maximized grid marked the size dirty")
	}
}

func TestOfflineResizeIsSavedAfterReconnect(t *testing.T) {
	m, _ := testModel(80, 24)
	m.connected = false
	m.sizeKnown = false
	m.windowOwned = true
	m.moveWindow = func(int, int) bool { return true }
	m.windowMaximizedFn = func() bool { return false }

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
	m.sendWindow = func(w, h int, pos *config.WindowPosition) error {
		sent = [2]int{w, h}
		return nil
	}

	m.flushWindow()
	if sent != [2]int{110, 35} {
		t.Fatalf("sent = %v, want 110x35", sent)
	}
	if m.windowDirty || m.windowSavedW != 110 || m.windowSavedH != 35 {
		t.Fatalf("unexpected state after flush: dirty=%v saved=%dx%d",
			m.windowDirty, m.windowSavedW, m.windowSavedH)
	}
}

func TestFlushWindowWithoutChanges(t *testing.T) {
	m, _ := testModel(80, 24)
	m.sizeKnown = true
	m.windowSupported = true
	m.windowSavedW, m.windowSavedH = 80, 24
	sent := false
	m.sendWindow = func(int, int, *config.WindowPosition) error {
		sent = true
		return nil
	}

	m.flushWindow()
	if sent {
		t.Fatal("an unchanged size must not be sent")
	}
}
