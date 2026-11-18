// Package updater is the short-lived helper that finishes an installer update.
// The daemon copies itself here, starts it hidden and exits. The helper waits
// for the daemon and the TUI to release their mutexes, runs the downloaded
// Inno Setup installer silently and reopens the new TUI.
package updater

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tuick-tack/internal/install"
	"tuick-tack/internal/instance"
	"tuick-tack/internal/notify"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/winproc"
)

const (
	// waitDaemon bounds how long the helper waits for the old daemon to exit.
	waitDaemon = 60 * time.Second
	// waitTUI bounds how long the helper waits for the TUI to exit.
	waitTUI = 30 * time.Second
)

// Apply runs the helper mode and returns the process exit code.
func Apply(args []string) int {
	fs := flag.NewFlagSet("apply-update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	setup := fs.String("setup", "", "downloaded setup executable")
	fallback := fs.String("fallback", "", "executable to reopen when no install is found")
	pipe := fs.String("wait-pipe", "", "daemon pipe name")
	relaunch := fs.Bool("relaunch", false, "open the TUI after the update")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	logger, closeLog := setupLog()
	defer closeLog()
	logger.Printf("apply: setup=%q fallback=%q pipe=%q relaunch=%v", *setup, *fallback, *pipe, *relaunch)

	if *setup == "" || *pipe == "" {
		logger.Printf("apply: missing setup or pipe")
		return 2
	}
	if _, err := os.Stat(*setup); err != nil {
		logger.Printf("apply: setup unavailable: %v", err)
		notifyFailure(logger)
		relaunchAfterFailure(logger, *fallback, *relaunch)
		return 1
	}

	daemonRelease, err := instance.WaitDaemonFree(*pipe, waitDaemon)
	if err != nil {
		logger.Printf("apply: waiting for daemon: %v", err)
		relaunchAfterFailure(logger, *fallback, *relaunch)
		return 1
	}
	tuiRelease, err := instance.WaitTUIFree(*pipe, waitTUI)
	if err != nil {
		logger.Printf("apply: waiting for TUI: %v", err)
		daemonRelease()
		relaunchAfterFailure(logger, *fallback, *relaunch)
		return 1
	}
	released := false
	releaseAll := func() {
		if released {
			return
		}
		released = true
		tuiRelease()
		daemonRelease()
	}
	defer releaseAll()

	runErr := runSetup(logger, *setup)
	// A new TUI starts the daemon itself, so both mutexes must be free before
	// the process is launched.
	releaseAll()

	target := installedExecutable(logger, *fallback)
	if runErr != nil {
		logger.Printf("apply: setup failed: %v", runErr)
		notifyFailure(logger)
		relaunchAfterFailure(logger, target, *relaunch)
		return 1
	}
	_ = os.Remove(*setup)
	if !*relaunch {
		logger.Printf("apply: done")
		return 0
	}
	if target == "" {
		logger.Printf("apply: no executable to reopen")
		notifyFailure(logger)
		return 1
	}
	if err := winproc.StartConsole(target); err != nil {
		logger.Printf("apply: reopen %s: %v", target, err)
		notifyFailure(logger)
		return 1
	}
	logger.Printf("apply: reopened %s", target)
	return 0
}

// Spawn copies the running executable to the cache and starts it as the update
// helper. The copy survives the installer replacing the original file.
func Spawn(setupPath, fallback, pipe string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dir, err := updateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	cleanupHelpers(dir)
	helper := filepath.Join(dir, fmt.Sprintf("apply-%d-%d.exe", os.Getpid(), time.Now().UnixNano()))
	if err := copyFile(self, helper); err != nil {
		return fmt.Errorf("copy update helper: %w", err)
	}
	if err := winproc.StartHidden(helper,
		"--apply-update",
		"--setup", setupPath,
		"--fallback", fallback,
		"--wait-pipe", pipe,
		"--relaunch",
	); err != nil {
		_ = os.Remove(helper)
		return err
	}
	return nil
}

// UpdateDir is where the daemon downloads update artifacts and helper copies.
func UpdateDir() (string, error) {
	return updateDir()
}

func updateDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "tuick-tack", "updates"), nil
}

func runSetup(logger *log.Logger, setupPath string) error {
	cache, err := os.UserCacheDir()
	if err == nil {
		logs := filepath.Join(cache, "tuick-tack", "logs")
		_ = os.MkdirAll(logs, 0o700)
	}
	args := []string{
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/CLOSEAPPLICATIONS",
	}
	if err == nil {
		args = append(args, `/LOG="`+filepath.Join(cache, "tuick-tack", "logs", "setup.log")+`"`)
	}
	cmd := exec.Command(setupPath, args...)
	cmd.Dir = filepath.Dir(setupPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	logger.Printf("setup: %s %s", setupPath, strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// installedExecutable returns the copy the installer registered, falling back
// to the executable that started the update and then to the default folder.
func installedExecutable(logger *log.Logger, fallback string) string {
	info, err := install.Find()
	if err != nil {
		logger.Printf("install lookup: %v", err)
	}
	candidates := []string{info.Exe, fallback}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "Documents", "tuick-tack", install.ExecutableName))
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return ""
}

func relaunchAfterFailure(logger *log.Logger, target string, relaunch bool) {
	if !relaunch || target == "" {
		return
	}
	if err := winproc.StartConsole(target); err != nil {
		logger.Printf("reopen after failure: %v", err)
		notifyFailure(logger)
	}
}

func notifyFailure(logger *log.Logger) {
	if err := notify.Show(copy.T(copy.NotifyUpdateFailedTitle), copy.T(copy.NotifyUpdateFailedBody), ""); err != nil {
		logger.Printf("notification: %v", err)
	}
}

func setupLog() (*log.Logger, func()) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return log.New(io.Discard, "", 0), func() {}
	}
	dir := filepath.Join(cache, "tuick-tack", "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return log.New(io.Discard, "", 0), func() {}
	}
	f, err := os.OpenFile(filepath.Join(dir, "update.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return log.New(io.Discard, "", 0), func() {}
	}
	return log.New(f, "", log.LstdFlags|log.Lmicroseconds), func() { f.Close() }
}

// cleanupHelpers removes helper copies left by updates older than an hour. A
// helper still running keeps its file locked, so failures are ignored.
func cleanupHelpers(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "apply-") || !strings.HasSuffix(entry.Name(), ".exe") {
			continue
		}
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < time.Hour {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}
