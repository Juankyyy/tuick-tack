package daemon

import (
	"strings"
	"time"

	"tuick-tack/internal/engine"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/stats"
)

// runTrack follows the active countdown so the history can record how a
// session really went: its wall-clock start, the planned duration after
// +/- adjustments and the time spent on hold. The engine stays untouched; this
// is wiring-level bookkeeping.
type runTrack struct {
	active   bool
	started  time.Time
	planned  time.Duration
	paused   time.Duration
	pausedAt time.Time
	label    string
	presetID string
}

// timestamp returns the daemon clock. Tests inject now; production uses the
// system clock.
func (d *Daemon) timestamp() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

// location is the time zone used to bucket local days and hours.
func (d *Daemon) location() *time.Location {
	if d.loc != nil {
		return d.loc
	}
	return time.Local
}

// beginRun starts tracking a countdown that just started.
func (d *Daemon) beginRun(started time.Time, planned time.Duration, label, presetID string) {
	d.trackMu.Lock()
	d.track = runTrack{
		active:   true,
		started:  started,
		planned:  planned,
		label:    strings.TrimSpace(label),
		presetID: presetID,
	}
	d.trackMu.Unlock()
}

// pauseRun freezes the pause clock. It is a no-op unless a run is active and
// running.
func (d *Daemon) pauseRun(now time.Time) {
	d.trackMu.Lock()
	if d.track.active && d.track.pausedAt.IsZero() {
		d.track.pausedAt = now
	}
	d.trackMu.Unlock()
}

// resumeRun banks the time spent on hold.
func (d *Daemon) resumeRun(now time.Time) {
	d.trackMu.Lock()
	if d.track.active && !d.track.pausedAt.IsZero() {
		if held := now.Sub(d.track.pausedAt); held > 0 {
			d.track.paused += held
		}
		d.track.pausedAt = time.Time{}
	}
	d.trackMu.Unlock()
}

// adjustRun moves the planned duration so the record reflects the final goal.
func (d *Daemon) adjustRun(delta time.Duration) {
	d.trackMu.Lock()
	if d.track.active {
		d.track.planned += delta
		if d.track.planned <= 0 {
			d.track.planned = time.Millisecond
		}
	}
	d.trackMu.Unlock()
}

// endRun writes the finished session to the history and clears the track. It
// ignores outcomes with no active run, so a repeated notice cannot duplicate a
// session.
func (d *Daemon) endRun(st engine.State, outcome stats.Outcome, now time.Time) {
	d.trackMu.Lock()
	t := d.track
	if !t.active {
		d.trackMu.Unlock()
		return
	}
	d.track = runTrack{}
	paused := t.paused
	if !t.pausedAt.IsZero() {
		if held := now.Sub(t.pausedAt); held > 0 {
			paused += held
		}
	}
	d.trackMu.Unlock()

	planned := t.planned
	if planned <= 0 {
		planned = st.Duration
	}
	if planned <= 0 {
		return
	}
	remaining := st.Remaining
	if remaining < 0 {
		remaining = 0
	}
	elapsed := planned - remaining
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > planned {
		elapsed = planned
	}
	label := t.label
	if label == "" {
		label = strings.TrimSpace(st.Label)
	}
	presetID := t.presetID
	if presetID == "" {
		presetID = st.PresetID
	}

	rec := stats.Record{
		StartedMS: t.started.UnixMilli(),
		PlannedMS: planned.Milliseconds(),
		ElapsedMS: elapsed.Milliseconds(),
		PausedMS:  paused.Milliseconds(),
		Label:     label,
		PresetID:  presetID,
		Outcome:   outcome,
	}
	if d.stats == nil {
		return
	}
	if err := d.stats.Add(rec); err != nil && d.logger != nil {
		d.logger.Printf("stats: %v", err)
	}
}

// clearRun drops the track without recording anything (dismiss, quit).
func (d *Daemon) clearRun() {
	d.trackMu.Lock()
	d.track = runTrack{}
	d.trackMu.Unlock()
}

// statsPayload aggregates the history for the requesting TUI.
func (d *Daemon) statsPayload() protocol.StatsPayload {
	if d.stats == nil {
		return protocol.StatsPayload{}
	}
	return d.stats.Aggregate(d.timestamp(), d.location())
}
