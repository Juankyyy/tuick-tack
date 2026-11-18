package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testLocation() *time.Location { return time.FixedZone("TST", 0) }

func testNow(loc *time.Location) time.Time {
	return time.Date(2026, 11, 18, 15, 0, 0, 0, loc)
}

func at(now time.Time, daysAgo, hour, minute int) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	return day.AddDate(0, 0, -daysAgo)
}

func rec(started time.Time, planned, elapsed time.Duration, label string, outcome Outcome) Record {
	return Record{
		StartedMS: started.UnixMilli(),
		PlannedMS: planned.Milliseconds(),
		ElapsedMS: elapsed.Milliseconds(),
		Label:     label,
		Outcome:   outcome,
	}
}

func TestStoreAppendsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	s := NewStore(path)
	now := testNow(testLocation())

	first := rec(at(now, 0, 9, 0), 25*time.Minute, 25*time.Minute, "Focus", Completed)
	second := rec(at(now, 1, 10, 0), 10*time.Minute, 4*time.Minute, "5 min", Stopped)
	if err := s.Add(first); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	if err := s.Add(second); err != nil {
		t.Fatalf("Add second: %v", err)
	}
	if s.Len() != 2 {
		t.Fatalf("Len = %d, want 2", s.Len())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "\n"); got != 2 {
		t.Fatalf("file lines = %d, want 2", got)
	}

	loaded := NewStore(path)
	skipped, err := loaded.Load()
	if err != nil || skipped != 0 {
		t.Fatalf("Load = %d, %v", skipped, err)
	}
	recs := loaded.Records()
	if len(recs) != 2 || recs[0] != first || recs[1] != second {
		t.Fatalf("reloaded records = %+v", recs)
	}
}

func TestAddRejectsInvalidRecord(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "history.jsonl"))
	if err := s.Add(Record{Label: "bad"}); err == nil {
		t.Fatal("invalid record was accepted")
	}
}

func TestStoreSkipsBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	now := testNow(testLocation())
	good, _ := json.Marshal(rec(at(now, 0, 9, 0), time.Minute, time.Minute, "Focus", Completed))
	invalid, _ := json.Marshal(Record{PlannedMS: 0, Outcome: Completed})
	content := string(good) + "\nnot json\n\n" + string(invalid) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	skipped, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if skipped != 2 {
		t.Fatalf("skipped = %d, want 2", skipped)
	}
	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1", s.Len())
	}
}

func TestStoreBacksUpUnreadableHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(path, []byte("garbage\nmore garbage\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	clock := func() time.Time { return time.Date(2026, 11, 18, 12, 0, 0, 0, testLocation()) }
	s := NewStoreWithClock(path, clock)
	if _, err := s.Load(); err == nil {
		t.Fatal("all-invalid history should report an error")
	}
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0", s.Len())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var backups int
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "history.bad-") && strings.HasSuffix(entry.Name(), ".jsonl") {
			backups++
		}
	}
	if backups != 1 {
		t.Fatalf("backups = %d, want 1", backups)
	}
	if data, err := os.ReadFile(path); err != nil || len(data) != 0 {
		t.Fatalf("history after backup = %d bytes, %v; want an empty file", len(data), err)
	}

	fresh := rec(at(testNow(testLocation()), 0, 9, 0), time.Minute, time.Minute, "Focus", Completed)
	if err := s.Add(fresh); err != nil {
		t.Fatalf("Add after backup: %v", err)
	}
	reloaded := NewStore(path)
	skipped, err := reloaded.Load()
	if err != nil || skipped != 0 || reloaded.Len() != 1 {
		t.Fatalf("reload after backup = skipped %d, err %v, len %d; want 0, nil, 1",
			skipped, err, reloaded.Len())
	}
}

func TestLoadTrimsToCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	now := testNow(testLocation())
	var b strings.Builder
	for i := 0; i < Cap+10; i++ {
		line, _ := json.Marshal(rec(at(now, 0, 9, i%60), time.Minute, time.Minute, "Focus", Completed))
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	if _, err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Len() != Cap {
		t.Fatalf("Len = %d, want %d", s.Len(), Cap)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "\n"); got != Cap {
		t.Fatalf("trimmed file lines = %d, want %d", got, Cap)
	}
}

func TestAggregate(t *testing.T) {
	loc := testLocation()
	now := testNow(loc)
	path := filepath.Join(t.TempDir(), "history.jsonl")
	s := NewStore(path)
	for _, r := range []Record{
		rec(at(now, 0, 9, 30), 25*time.Minute, 25*time.Minute, "Focus", Completed),
		rec(at(now, 0, 10, 30), 25*time.Minute, 10*time.Minute, "5 min", Stopped),
		rec(at(now, 1, 14, 0), 50*time.Minute, 50*time.Minute, "Focus", Completed),
		rec(at(now, 2, 14, 0), 25*time.Minute, 5*time.Minute, "25 min", Replaced),
		rec(at(now, 3, 12, 0), 10*time.Minute, 10*time.Minute, "10 min", Completed),
		rec(at(now, 10, 12, 0), time.Hour, time.Hour, "Focus", Completed),
		rec(at(now, 40, 12, 0), 30*time.Minute, 30*time.Minute, "Deep", Completed),
		rec(at(now, 100, 12, 0), 15*time.Minute, 15*time.Minute, "5 min", Completed),
	} {
		if err := s.Add(r); err != nil {
			t.Fatal(err)
		}
	}

	p := s.Aggregate(now, loc)
	if p.Retained != 8 {
		t.Fatalf("Retained = %d, want 8", p.Retained)
	}
	if len(p.Daily) != DailyWindow || len(p.Hourly) != 24 || len(p.Weekday) != 7 {
		t.Fatalf("payload shape: daily=%d hourly=%d weekday=%d",
			len(p.Daily), len(p.Hourly), len(p.Weekday))
	}
	if got, want := p.Daily[len(p.Daily)-1].Date, "2026-11-18"; got != want {
		t.Fatalf("last daily date = %q, want %q", got, want)
	}
	if got, want := p.Daily[len(p.Daily)-1].MS, int64(35*time.Minute/time.Millisecond); got != want {
		t.Fatalf("today MS = %d, want %d", got, want)
	}
	if got, want := p.Daily[len(p.Daily)-1].N, 2; got != want {
		t.Fatalf("today N = %d, want %d", got, want)
	}
	if got, want := p.Daily[len(p.Daily)-2].MS, int64(50*time.Minute/time.Millisecond); got != want {
		t.Fatalf("yesterday MS = %d, want %d", got, want)
	}

	d7 := p.Totals.D7
	if d7.Sessions != 5 || d7.Completed != 3 || d7.Stopped != 1 || d7.Replaced != 1 {
		t.Fatalf("D7 = %+v", d7)
	}
	if got, want := d7.MS, int64(100*time.Minute/time.Millisecond); got != want {
		t.Fatalf("D7 MS = %d, want %d", got, want)
	}
	if got := p.Totals.D30.Sessions; got != 6 {
		t.Fatalf("D30 sessions = %d, want 6", got)
	}
	if got := p.Totals.D90.Sessions; got != 7 {
		t.Fatalf("D90 sessions = %d, want 7", got)
	}
	if p.Totals.All.Sessions != 8 {
		t.Fatalf("All sessions = %d, want 8", p.Totals.All.Sessions)
	}

	if got, want := p.Labels[0].Label, "Focus"; got != want {
		t.Fatalf("top label = %q, want %q", got, want)
	}
	if got, want := p.Labels[0].MS, int64(135*time.Minute/time.Millisecond); got != want {
		t.Fatalf("top label MS = %d, want %d", got, want)
	}
	if p.Labels[0].Completed != 3 {
		t.Fatalf("top label completed = %d, want 3", p.Labels[0].Completed)
	}
	if len(p.Labels) != 5 {
		t.Fatalf("labels = %d, want 5", len(p.Labels))
	}

	if got, want := p.Records.StreakDays, 4; got != want {
		t.Fatalf("streak = %d, want %d", got, want)
	}
	if got, want := p.Records.BestStreakDays, 4; got != want {
		t.Fatalf("best streak = %d, want %d", got, want)
	}
	if got, want := p.Records.LongestMS, int64(time.Hour/time.Millisecond); got != want {
		t.Fatalf("longest = %d, want %d", got, want)
	}
	if p.Records.LongestLabel != "Focus" {
		t.Fatalf("longest label = %q", p.Records.LongestLabel)
	}
	if got, want := p.Records.BusiestHour, 12; got != want {
		t.Fatalf("busiest hour = %d, want %d", got, want)
	}
	// The 10-day record (60 min) plus the 3-day one (10 min) land on Sunday.
	if got := p.Records.BusiestWeekday; got != 6 {
		t.Fatalf("busiest weekday = %d, want 6 (Sunday)", got)
	}

	if len(p.Recent) != 8 {
		t.Fatalf("recent = %d, want 8", len(p.Recent))
	}
	if p.Recent[0].Label != "5 min" || p.Recent[0].Outcome != string(Stopped) {
		t.Fatalf("recent[0] = %+v", p.Recent[0])
	}
	if p.Recent[len(p.Recent)-1].Label != "5 min" {
		t.Fatalf("recent[last] = %+v", p.Recent[len(p.Recent)-1])
	}
}

func TestAggregateEmpty(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "history.jsonl"))
	p := s.Aggregate(testNow(testLocation()), testLocation())
	if p.Retained != 0 || len(p.Recent) != 0 || len(p.Labels) != 0 {
		t.Fatalf("empty history produced %+v", p)
	}
	if p.Records.BusiestHour != -1 || p.Records.BusiestWeekday != -1 {
		t.Fatalf("empty history has a busiest slot: %+v", p.Records)
	}
	if p.Records.StreakDays != 0 || p.Records.BestStreakDays != 0 {
		t.Fatalf("empty history has a streak: %+v", p.Records)
	}
	if len(p.Daily) != DailyWindow {
		t.Fatalf("daily = %d, want %d", len(p.Daily), DailyWindow)
	}
}

func TestCurrentStreakSurvivesAnEmptyMorning(t *testing.T) {
	loc := testLocation()
	today := time.Date(2026, 11, 18, 12, 0, 0, 0, loc)
	days := map[string]bool{
		today.AddDate(0, 0, -1).Format(dateLayout): true,
		today.AddDate(0, 0, -2).Format(dateLayout): true,
	}
	if got, want := currentStreak(days, today), 2; got != want {
		t.Fatalf("streak = %d, want %d", got, want)
	}
	if got := currentStreak(map[string]bool{today.AddDate(0, 0, -2).Format(dateLayout): true}, today); got != 0 {
		t.Fatalf("broken streak = %d, want 0", got)
	}
}

func TestDailyWindowExcludesOlderDays(t *testing.T) {
	loc := testLocation()
	now := testNow(loc)
	s := NewStore(filepath.Join(t.TempDir(), "history.jsonl"))
	_ = s.Add(rec(at(now, DailyWindow, 12, 0), time.Minute, time.Minute, "Old", Completed))
	p := s.Aggregate(now, loc)
	if p.Totals.All.Sessions != 1 {
		t.Fatalf("all sessions = %d", p.Totals.All.Sessions)
	}
	if p.Totals.D90.Sessions != 0 {
		t.Fatalf("a record older than the window leaked into D90")
	}
	if got := p.Daily[0].N; got != 0 {
		t.Fatalf("old record landed on the first daily slot")
	}
}
