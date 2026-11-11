package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store reads and writes config.json. Save is atomic: the payload goes to a
// temporary file in the same directory and is renamed over the target.
type Store struct {
	path string
	now  func() time.Time
}

// NewStore returns a store for the given config file path.
func NewStore(path string) *Store {
	return &Store{path: path, now: time.Now}
}

// NewStoreWithClock is the test hook for the backup timestamp.
func NewStoreWithClock(path string, now func() time.Time) *Store {
	return &Store{path: path, now: now}
}

// DefaultPath returns %APPDATA%\tuick-tack\config.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tuick-tack", "config.json"), nil
}

// Path returns the config file path.
func (s *Store) Path() string { return s.path }

// Load returns the stored configuration. A missing file yields defaults
// without an error. A corrupt or invalid file is renamed to
// config.bad-<timestamp>.json and defaults are returned with ErrInvalid (or
// the underlying I/O error) so the caller can log it and continue.
func (s *Store) Load() (Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return s.recover(data, fmt.Errorf("%w: %v", ErrInvalid, err))
	}
	c = Normalize(c)
	if err := Validate(c); err != nil {
		return s.recover(data, fmt.Errorf("%w: %v", ErrInvalid, err))
	}
	return c, nil
}

func (s *Store) recover(original []byte, cause error) (Config, error) {
	stamp := s.now().Format("20060102-150405.000")
	backup := filepath.Join(filepath.Dir(s.path), fmt.Sprintf("config.bad-%s.json", stamp))
	for i := 1; i < 100; i++ {
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			break
		}
		backup = filepath.Join(filepath.Dir(s.path),
			fmt.Sprintf("config.bad-%s-%d.json", stamp, i))
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return Default(), fmt.Errorf("backup config: %w", err)
	}
	if err := os.WriteFile(backup, original, 0o600); err != nil {
		return Default(), fmt.Errorf("backup config: %w", err)
	}
	return Default(), cause
}

// Save validates and atomically writes the configuration.
func (s *Store) Save(c Config) error {
	c = Normalize(c)
	if err := Validate(c); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
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
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	return nil
}
