package engine

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time          { return f.t }
func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func newTestEngine() (*Engine, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 11, 11, 9, 0, 0, 0, time.Local)}
	return New(clock), clock
}

func TestStartAndCountdown(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(5*time.Minute, "5 min", "p5")

	st := e.Snapshot()
	if st.Status != Running || st.Duration != 5*time.Minute || st.Remaining != 5*time.Minute {
		t.Fatalf("unexpected start state: %+v", st)
	}
	if st.EndAt.IsZero() {
		t.Fatal("running state must expose an end time")
	}

	clock.Advance(90 * time.Second)
	st = e.Snapshot()
	if st.Remaining != 3*time.Minute+30*time.Second {
		t.Fatalf("remaining = %v, want 3m30s", st.Remaining)
	}
}

func TestPauseResume(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(10*time.Minute, "10 min", "")
	clock.Advance(2 * time.Minute)

	if !e.Pause() {
		t.Fatal("Pause returned false")
	}
	st := e.Snapshot()
	if st.Status != Paused || st.Remaining != 8*time.Minute {
		t.Fatalf("unexpected paused state: %+v", st)
	}
	if !st.EndAt.IsZero() {
		t.Fatal("paused state must not expose an end time")
	}

	clock.Advance(time.Hour)
	if got := e.Snapshot().Remaining; got != 8*time.Minute {
		t.Fatalf("paused remaining moved: %v", got)
	}
	if e.Pause() {
		t.Fatal("Pause on paused engine must be a no-op")
	}

	if !e.Resume() {
		t.Fatal("Resume returned false")
	}
	clock.Advance(time.Minute)
	st = e.Snapshot()
	if st.Status != Running || st.Remaining != 7*time.Minute {
		t.Fatalf("unexpected resumed state: %+v", st)
	}
	if e.Resume() {
		t.Fatal("Resume on running engine must be a no-op")
	}
}

func TestFinishOnce(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(time.Minute, "1 min", "")
	clock.Advance(time.Minute)

	st, crossed := e.Tick()
	if !crossed {
		t.Fatal("expected the first Tick to cross zero")
	}
	if st.Status != Finished || st.Remaining != 0 || st.FinishSeq != 1 {
		t.Fatalf("unexpected finished state: %+v", st)
	}
	clock.Advance(time.Hour)
	st, crossed = e.Tick()
	if crossed {
		t.Fatal("finish fired more than once")
	}
	if st.FinishSeq != 1 {
		t.Fatalf("finish seq = %d, want 1", st.FinishSeq)
	}
}

func TestPauseAtDeadlineFinishes(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(30*time.Second, "", "")
	clock.Advance(30 * time.Second)
	e.Pause()
	st := e.Snapshot()
	if st.Status != Finished || st.FinishSeq != 1 {
		t.Fatalf("pause at deadline should finish: %+v", st)
	}
}

func TestAdjust(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(5*time.Minute, "", "")

	e.Adjust(time.Minute)
	if got := e.Snapshot().Remaining; got != 6*time.Minute {
		t.Fatalf("running adjust: %v, want 6m", got)
	}
	e.Adjust(-2 * time.Minute)
	if got := e.Snapshot().Remaining; got != 4*time.Minute {
		t.Fatalf("running adjust: %v, want 4m", got)
	}

	e.Pause()
	e.Adjust(time.Minute)
	if got := e.Snapshot().Remaining; got != 5*time.Minute {
		t.Fatalf("paused adjust: %v, want 5m", got)
	}

	clock.Advance(10 * time.Second)
	_, crossed := e.Tick()
	if crossed {
		t.Fatal("paused timer must not finish by ticking")
	}

	e.Adjust(-10 * time.Minute)
	st := e.Snapshot()
	if st.Status != Finished || st.FinishSeq != 1 {
		t.Fatalf("adjust to zero should finish: %+v", st)
	}
	if e.Adjust(time.Minute) {
		t.Fatal("adjust in finished state must be a no-op")
	}
}

func TestAdjustRunningToZero(t *testing.T) {
	e, _ := newTestEngine()
	e.Start(30*time.Second, "", "")
	e.Adjust(-time.Minute)
	st := e.Snapshot()
	if st.Status != Finished || st.FinishSeq != 1 {
		t.Fatalf("unexpected state: %+v", st)
	}
}

func TestReplaceDoesNotCountAsFinish(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(5*time.Minute, "five", "p5")
	clock.Advance(time.Minute)
	e.Start(10*time.Minute, "ten", "p10")

	st := e.Snapshot()
	if st.Status != Running || st.Duration != 10*time.Minute || st.FinishSeq != 0 {
		t.Fatalf("unexpected replaced state: %+v", st)
	}
	if st.Label != "ten" || st.PresetID != "p10" {
		t.Fatalf("labels not replaced: %+v", st)
	}
}

func TestRestartAndDismiss(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(2*time.Minute, "two", "p2")
	clock.Advance(2 * time.Minute)
	e.Tick()
	if !e.Dismiss() {
		t.Fatal("Dismiss on finished returned false")
	}
	st := e.Snapshot()
	if st.Status != Idle || st.Duration != 0 || st.Label != "" {
		t.Fatalf("unexpected idle state: %+v", st)
	}
	if e.Dismiss() {
		t.Fatal("Dismiss on idle must be a no-op")
	}
	if !e.Restart() {
		t.Fatal("Restart returned false")
	}
	st = e.Snapshot()
	if st.Status != Running || st.Duration != 2*time.Minute || st.Label != "two" {
		t.Fatalf("unexpected restarted state: %+v", st)
	}
	if st.FinishSeq != 1 {
		t.Fatalf("restart must preserve finish seq, got %d", st.FinishSeq)
	}
}

func TestStop(t *testing.T) {
	e, _ := newTestEngine()
	e.Start(time.Minute, "one", "p1")
	e.Stop()
	st := e.Snapshot()
	if st.Status != Idle || st.Duration != 0 || st.Remaining != 0 {
		t.Fatalf("unexpected stopped state: %+v", st)
	}
	if !e.Restart() {
		t.Fatal("restart after stop should reuse the last duration")
	}
}

func TestZeroAndNegativeRemainingSnapshot(t *testing.T) {
	e, clock := newTestEngine()
	e.Start(time.Second, "", "")
	clock.Advance(2 * time.Second)
	st := e.Snapshot()
	if st.Remaining != 0 {
		t.Fatalf("remaining = %v, want 0", st.Remaining)
	}
	if st.Status != Running {
		t.Fatalf("snapshot must not finalize before Tick: %+v", st)
	}
}
