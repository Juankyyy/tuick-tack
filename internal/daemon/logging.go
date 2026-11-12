package daemon

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"tuick-tack/internal/buildinfo"
)

// CacheDir returns %LOCALAPPDATA%\tuick-tack (os.UserCacheDir).
func CacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tuick-tack"), nil
}

// SetupLogging creates the per-user log file and returns a logger, a closer
// and the cache directory. A failure to open the log never prevents the daemon
// from running: it falls back to a discarding logger so a daemon without a
// console stays quiet.
func SetupLogging() (*log.Logger, func(), string, error) {
	cache, cacheErr := CacheDir()
	logDir := filepath.Join(cache, "logs")
	if cacheErr == nil {
		if err := os.MkdirAll(logDir, 0o700); err != nil {
			cacheErr = err
		}
	}
	if cacheErr != nil {
		return log.New(io.Discard, "", 0), func() {}, "", cacheErr
	}
	f, err := os.OpenFile(filepath.Join(logDir, "tuick-tack.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return log.New(io.Discard, "", 0), func() {}, cache, err
	}
	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	return logger, func() { f.Close() }, cache, nil
}

// LogStartup writes the standard banner so a support log starts with context.
func LogStartup(logger *log.Logger) {
	logger.Printf("%s %s starting (pid %d)", buildinfo.Name, buildinfo.Version, os.Getpid())
	logger.Printf("time: %s", time.Now().Format(time.RFC3339))
	if exe, err := os.Executable(); err == nil {
		logger.Printf("executable: %s", exe)
	}
	if dir, err := os.UserConfigDir(); err == nil {
		logger.Printf("config dir: %s", dir)
	}
	if dir, err := os.UserCacheDir(); err == nil {
		logger.Printf("cache dir: %s", dir)
	}
}

func logError(logger *log.Logger, format string, args ...any) {
	if logger != nil {
		logger.Printf("error: "+format, args...)
	} else {
		fmt.Fprintf(io.Discard, format, args...)
	}
}
