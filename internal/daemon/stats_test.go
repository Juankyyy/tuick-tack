package daemon

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/stats"
)

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time          { return c.t }
func (c *fixedClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func testDaemon(t *testing.T) (*Daemon, *stats.Store, *fixedClock) {
	t.Helper()
	clock := &fixedClock{t: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)}
	store := stats.NewStore(filepath.Join(t.TempDir(), "history.jsonl"))
	cfg := config.Default()
	cfg.Sound = false
	d := &Daemon{
		logger: log.New(io.Discard, "", 0),
		engine: engine.New(clock),
		stats:  store,
		now:    clock.Now,
		loc:    time.UTC,
		cfg:    cfg,
	}
	return d, store, clock
}

func TestStatsTrackPausedSession(t *testing.T) {
	d, store, clock := testDaemon(t)
	d.startTimer(60, "1 min", "p1")
	if d.stats.Len() != 0 {
		t.Fatal("starting a timer recorded a session")
	}

	clock.Advance(10 * time.Second)
	d.TogglePause()
	if d.engine.Snapshot().Status != engine.Paused {
		t.Fatal("timer did not pause")
	}
	clock.Advance(5 * time.Second)
	d.TogglePause()
	clock.Advance(5 * time.Second)
	d.stopTimer()

	recs := d.stats.Records()
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	rec := recs[0]
	if rec.Outcome != stats.Stopped {
		t.Fatalf("outcome = %q, want stopped", rec.Outcome)
	}
	if rec.PlannedMS != 60000 || rec.ElapsedMS != 15000 || rec.PausedMS != 5000 {
		t.Fatalf("record = %+v", rec)
	}
	if rec.Label != "1 min" || rec.PresetID != "p1" {
		t.Fatalf("label/preset = %q/%q", rec.Label, rec.PresetID)
	}
	if store.Len() != 1 {
		t.Fatalf("store rows = %d", store.Len())
	}
}

func TestStatsReplaceAndFinish(t *testing.T) {
	d, store, clock := testDaemon(t)
	d.startTimer(300, "5 min", "p5")
	clock.Advance(2 * time.Minute)
	d.startTimer(600, "10 min", "p10")

	recs := d.stats.Records()
	if len(recs) != 1 || recs[0].Outcome != stats.Replaced {
		t.Fatalf("replaced record = %+v", recs)
	}
	if recs[0].PlannedMS != 300000 || recs[0].ElapsedMS != 120000 {
		t.Fatalf("replaced times = %+v", recs[0])
	}

	d.onFinish(engine.State{
		Status:   engine.Finished,
		Duration: 10 * time.Minute,
		Label:    "10 min",
		PresetID: "p10",
	})
	if store.Len() != 2 {
		t.Fatalf("store rows = %d, want 2", store.Len())
	}
	done := store.Records()[1]
	if done.Outcome != stats.Completed || done.PlannedMS != 600000 ||
		done.ElapsedMS != 600000 || done.Label != "10 min" {
		t.Fatalf("completed record = %+v", done)
	}
	// A repeated finish notice must not duplicate the session.
	d.onFinish(engine.State{Status: engine.Finished, Duration: 10 * time.Minute, Label: "10 min"})
	if store.Len() != 2 {
		t.Fatalf("repeated finish wrote %d records", store.Len())
	}
}

func TestStatsAdjustChangesPlanned(t *testing.T) {
	d, _, clock := testDaemon(t)
	d.startTimer(25*60, "25 min", "p25")
	clock.Advance(time.Minute)
	d.adjustRun(time.Minute)
	d.endRun(engine.State{
		Status:    engine.Paused,
		Duration:  25 * time.Minute,
		Remaining: 25 * time.Minute,
		Label:     "25 min",
	}, stats.Stopped, clock.Now())

	rec := d.stats.Records()[0]
	if rec.PlannedMS != 26*60000 {
		t.Fatalf("planned = %d, want %d", rec.PlannedMS, 26*60000)
	}
	if rec.ElapsedMS != 60000 {
		t.Fatalf("elapsed = %d, want 60000", rec.ElapsedMS)
	}
}

func TestStatsPayloadFromStore(t *testing.T) {
	d, store, clock := testDaemon(t)
	d.beginRun(clock.Now(), 5*time.Minute, "5 min", "p5")
	d.endRun(engine.State{Status: engine.Finished, Duration: 5 * time.Minute, Label: "5 min"},
		stats.Completed, clock.Now())

	payload := d.statsPayload()
	if payload.Retained != 1 || len(payload.Recent) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Records.StreakDays != 1 {
		t.Fatalf("streak = %d, want 1", payload.Records.StreakDays)
	}
	if store.Len() != 1 {
		t.Fatalf("store rows = %d", store.Len())
	}

	empty := &Daemon{loc: time.UTC}
	if p := empty.statsPayload(); p.Retained != 0 {
		t.Fatalf("empty daemon payload = %+v", p)
	}
}

func TestGetStatsOverPipe(t *testing.T) {
	d, store, clock := testDaemon(t)
	d.beginRun(clock.Now(), 5*time.Minute, "5 min", "p5")
	d.endRun(engine.State{Status: engine.Finished, Duration: 5 * time.Minute, Label: "5 min"},
		stats.Completed, clock.Now())
	if store.Len() != 1 {
		t.Fatalf("store rows = %d", store.Len())
	}

	pipe := fmt.Sprintf(`\\.\pipe\tuick-tack-test-stats-%d-%d`, os.Getpid(), time.Now().UnixNano())
	server := ipc.NewServer(pipe, "test", d)
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Close()

	client, _, err := ipc.Dial(pipe)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	if err := client.Send(protocol.ClientMessage{Type: protocol.TypeGetStats}); err != nil {
		t.Fatalf("send get_stats: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		msg, err := client.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if statsMsg, ok := msg.(protocol.StatsMessage); ok {
			if statsMsg.Stats.Retained != 1 || len(statsMsg.Stats.Recent) != 1 {
				t.Fatalf("stats reply = %+v", statsMsg.Stats)
			}
			if statsMsg.Stats.Records.StreakDays != 1 {
				t.Fatalf("streak = %d, want 1", statsMsg.Stats.Records.StreakDays)
			}
			return
		}
	}
}
