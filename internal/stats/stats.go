// Package stats keeps the local history of finished countdowns and turns it
// into the summaries the Statistics tab renders.
//
// The daemon owns the store and is its only writer. Records live in an
// append-only JSONL file so a torn line never loses the rest of the history,
// and the file is trimmed atomically once it grows past the cap. This is user
// data, not configuration: config.json stays the single settings store.
package stats

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// Cap is how many records are kept. At a few sessions a day it is years of
	// history while staying a few hundred KiB.
	Cap = 2000
	// trimSlack lets the file grow past Cap before the rewrite, so trimming is
	// amortized instead of happening on every append.
	trimSlack = 256
	// DailyWindow is the number of local days the daemon sends to the TUI.
	// It covers the 53-week year heatmap (371 days), so the oldest Monday
	// column is complete whatever day of the week it is.
	DailyWindow = 371
	// RecentCount is how many sessions the recent list carries.
	RecentCount = 20
	// LabelCount is how many labels the presets section shows.
	LabelCount = 8

	dateLayout = "2006-01-02"
)

// Outcome says how a countdown ended.
type Outcome string

const (
	// Completed reached zero, including an adjustment that hit zero.
	Completed Outcome = "completed"
	// Stopped was cancelled by the user before zero.
	Stopped Outcome = "stopped"
	// Replaced was superseded by a new countdown.
	Replaced Outcome = "replaced"
)

// Valid reports whether o is one of the known outcomes.
func (o Outcome) Valid() bool {
	switch o {
	case Completed, Stopped, Replaced:
		return true
	}
	return false
}

// Record is one finished countdown. Elapsed is real run time (pauses
// excluded); Planned includes adjustments made during the session.
type Record struct {
	StartedMS int64   `json:"started_ms"`
	PlannedMS int64   `json:"planned_ms"`
	ElapsedMS int64   `json:"elapsed_ms"`
	PausedMS  int64   `json:"paused_ms,omitempty"`
	Label     string  `json:"label"`
	PresetID  string  `json:"preset_id,omitempty"`
	Outcome   Outcome `json:"outcome"`
}

func (r Record) valid() bool {
	return r.StartedMS > 0 && r.PlannedMS > 0 && r.ElapsedMS >= 0 && r.Outcome.Valid()
}

// Store is the in-memory history backed by one JSONL file. It is safe for
// concurrent use; only the daemon writes to it.
type Store struct {
	mu      sync.Mutex
	path    string
	now     func() time.Time
	records []Record
}

// NewStore returns a store for path.
func NewStore(path string) *Store {
	return &Store{path: path, now: time.Now}
}

// NewStoreWithClock is the test hook for the backup timestamp.
func NewStoreWithClock(path string, now func() time.Time) *Store {
	return &Store{path: path, now: now}
}

// DefaultPath returns %APPDATA%\tuick-tack\history.jsonl next to config.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tuick-tack", "history.jsonl"), nil
}

// Path returns the history file path.
func (s *Store) Path() string { return s.path }

// Load reads the history file into memory. A missing file is an empty history.
// Invalid lines are skipped and counted; when the file has content but no
// readable record it is backed up as history.bad-<timestamp>.jsonl and the
// store starts empty, mirroring the config recovery policy.
func (s *Store) Load() (skipped int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = nil
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	nonEmpty := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil || !rec.valid() {
			skipped++
			continue
		}
		s.records = append(s.records, rec)
	}

	if nonEmpty > 0 && len(s.records) == 0 {
		if err := s.backup(data); err != nil {
			return skipped, fmt.Errorf("stats: backup history: %w", err)
		}
		// Empty the live file too. Otherwise every future append would mix with
		// the damaged lines and every start would skip them again.
		if err := s.rewriteLocked(); err != nil {
			return skipped, fmt.Errorf("stats: reset history: %w", err)
		}
		return skipped, fmt.Errorf("stats: history had %d unreadable records; starting empty", nonEmpty)
	}
	if len(s.records) > Cap {
		s.records = append([]Record(nil), s.records[len(s.records)-Cap:]...)
		if err := s.rewriteLocked(); err != nil {
			return skipped, err
		}
	}
	return skipped, nil
}

func (s *Store) backup(original []byte) error {
	stamp := s.now().Format("20060102-150405.000")
	dir := filepath.Dir(s.path)
	backup := filepath.Join(dir, fmt.Sprintf("history.bad-%s.jsonl", stamp))
	for i := 1; i < 100; i++ {
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			break
		}
		backup = filepath.Join(dir, fmt.Sprintf("history.bad-%s-%d.jsonl", stamp, i))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(backup, original, 0o600)
}

// Add appends one record to memory and to the file. A record that fails
// validation is rejected instead of poisoning the history.
func (s *Store) Add(rec Record) error {
	if !rec.valid() {
		return fmt.Errorf("stats: invalid record %+v", rec)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(line, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}

	s.records = append(s.records, rec)
	if len(s.records) > Cap+trimSlack {
		s.records = append([]Record(nil), s.records[len(s.records)-Cap:]...)
		return s.rewriteLocked()
	}
	return nil
}

func (s *Store) rewriteLocked() error {
	var payload []byte
	for _, rec := range s.records {
		line, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		payload = append(payload, line...)
		payload = append(payload, '\n')
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".history-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

// Records returns a copy of the retained history, oldest first.
func (s *Store) Records() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Record(nil), s.records...)
}

// Len returns how many records are retained.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}
