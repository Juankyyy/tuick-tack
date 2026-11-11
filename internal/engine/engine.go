// Package engine implements the single countdown state machine.
//
// It owns no goroutine and reads time only through an injected clock, which
// keeps pause/resume/finish deterministic in tests. The daemon drives Tick at
// 100 ms and broadcasts snapshots.
package engine

import (
	"sync"
	"time"
)

// Status is the lifecycle state of the countdown.
type Status string

const (
	Idle     Status = "idle"
	Running  Status = "running"
	Paused   Status = "paused"
	Finished Status = "finished"
)

// Clock supplies the current time. Production uses systemClock; tests inject a
// fake clock.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// SystemClock returns the real clock.
func SystemClock() Clock { return systemClock{} }

// State is an immutable snapshot of the engine.
type State struct {
	Status     Status
	Duration   time.Duration
	Remaining  time.Duration
	Label      string
	PresetID   string
	FinishSeq  int64
	EndAt      time.Time
	LastLabel  string
	LastPreset string
}

// Engine is safe for concurrent use.
type Engine struct {
	mu     sync.Mutex
	clock  Clock
	status Status

	duration  time.Duration
	remaining time.Duration
	deadline  time.Time
	label     string
	presetID  string
	finishSeq int64

	lastDuration time.Duration
	lastLabel    string
	lastPresetID string
}

// New returns an idle engine using clock.
func New(clock Clock) *Engine {
	if clock == nil {
		clock = SystemClock()
	}
	return &Engine{clock: clock, status: Idle}
}

// Start begins a countdown, replacing any active one. The finished sequence is
// intentionally left untouched: replacing a timer is not a completion.
func (e *Engine) Start(d time.Duration, label, presetID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.startLocked(d, label, presetID)
}

func (e *Engine) startLocked(d time.Duration, label, presetID string) {
	now := e.clock.Now()
	e.status = Running
	e.duration = d
	e.remaining = d
	e.deadline = now.Add(d)
	e.label = label
	e.presetID = presetID
	e.lastDuration = d
	e.lastLabel = label
	e.lastPresetID = presetID
}

// Pause freezes the remaining time. It reports whether the call changed state.
func (e *Engine) Pause() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status != Running {
		return false
	}
	now := e.clock.Now()
	left := e.deadline.Sub(now)
	if left <= 0 {
		e.finishLocked()
		return true
	}
	e.remaining = left
	e.deadline = time.Time{}
	e.status = Paused
	return true
}

// Resume continues a paused countdown.
func (e *Engine) Resume() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status != Paused {
		return false
	}
	if e.remaining <= 0 {
		e.finishLocked()
		return true
	}
	e.deadline = e.clock.Now().Add(e.remaining)
	e.status = Running
	return true
}

// Stop returns to idle and clears the visible countdown. The last duration is
// retained for Restart.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.status = Idle
	e.duration = 0
	e.remaining = 0
	e.deadline = time.Time{}
	e.label = ""
	e.presetID = ""
}

// Dismiss returns a finished countdown to idle. It is a no-op in other states.
func (e *Engine) Dismiss() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status != Finished {
		return false
	}
	e.status = Idle
	e.duration = 0
	e.remaining = 0
	e.deadline = time.Time{}
	e.label = ""
	e.presetID = ""
	return true
}

// Restart runs the last used duration and label again.
func (e *Engine) Restart() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lastDuration <= 0 {
		return false
	}
	e.startLocked(e.lastDuration, e.lastLabel, e.lastPresetID)
	return true
}

// Adjust moves the deadline (running) or the frozen remaining value (paused)
// by delta. Reaching zero finalizes the countdown.
func (e *Engine) Adjust(delta time.Duration) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch e.status {
	case Running:
		e.deadline = e.deadline.Add(delta)
		if !e.deadline.After(e.clock.Now()) {
			e.finishLocked()
			return true
		}
		e.remaining = e.deadline.Sub(e.clock.Now())
		return true
	case Paused:
		e.remaining += delta
		if e.remaining <= 0 {
			e.finishLocked()
			return true
		}
		return true
	default:
		return false
	}
}

// Tick finalizes the countdown when the deadline has passed. The bool reports
// that this call crossed zero; it can only be true once per countdown.
func (e *Engine) Tick() (State, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status == Running && !e.deadline.After(e.clock.Now()) {
		e.finishLocked()
		return e.snapshotLocked(), true
	}
	return e.snapshotLocked(), false
}

func (e *Engine) finishLocked() {
	if e.status == Finished {
		return
	}
	e.status = Finished
	e.finishSeq++
	e.remaining = 0
	e.deadline = time.Time{}
}

// Snapshot returns the current state without changing it.
func (e *Engine) Snapshot() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked()
}

func (e *Engine) snapshotLocked() State {
	now := e.clock.Now()
	st := State{
		Status:     e.status,
		Duration:   e.duration,
		Label:      e.label,
		PresetID:   e.presetID,
		FinishSeq:  e.finishSeq,
		LastLabel:  e.lastLabel,
		LastPreset: e.lastPresetID,
	}
	switch e.status {
	case Running:
		left := e.deadline.Sub(now)
		if left < 0 {
			left = 0
		}
		st.Remaining = left
		st.EndAt = e.deadline
	case Paused:
		st.Remaining = e.remaining
	case Finished:
		st.Remaining = 0
	}
	return st
}
