package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/update"
	"tuick-tack/internal/updater"
)

const (
	updateKindSetup    = "setup"
	updateKindPortable = "portable"

	updatePhaseDownloading = "downloading"
	updatePhaseVerifying   = "verifying"
	updatePhaseInstalling  = "installing"
	updatePhaseDone        = "done"
	updatePhaseFailed      = "failed"

	updateTimeout = 10 * time.Minute
)

// cachedUpdate copies the latest release the daemon knows about.
func (d *Daemon) cachedUpdate() (update.Info, bool) {
	d.updateMu.RLock()
	defer d.updateMu.RUnlock()
	if !d.updateOK {
		return update.Info{}, false
	}
	return d.updateInfo, true
}

// installUpdate downloads the Inno Setup installer and starts the helper that
// runs it after the daemon and TUI exit. force is true only after the TUI has
// warned about an active countdown.
func (d *Daemon) installUpdate(c *ipc.Session, force bool) {
	info, ok := d.cachedUpdate()
	if !ok || !info.CanInstall() {
		d.updateProgress(updateKindSetup, updatePhaseFailed, 0, 0, "", "unavailable")
		return
	}
	if buildinfo.Version == "dev" {
		d.updateProgress(updateKindSetup, updatePhaseFailed, 0, 0, "", "unsupported")
		return
	}
	if d.timerActive() && !force {
		d.updateProgress(updateKindSetup, updatePhaseFailed, 0, 0, "", "timer_active")
		return
	}

	d.applyMu.Lock()
	if d.updating {
		d.applyMu.Unlock()
		d.updateProgress(updateKindSetup, updatePhaseFailed, 0, 0, "", "busy")
		return
	}
	if d.timerActive() && !force {
		d.applyMu.Unlock()
		d.updateProgress(updateKindSetup, updatePhaseFailed, 0, 0, "", "timer_active")
		return
	}
	d.updating = true
	d.applyMu.Unlock()

	asset, _ := info.SetupAsset()
	checksums, hasChecksums := info.ChecksumsAsset()
	dir, err := d.updateDirectory()
	if err != nil {
		d.failInstall("write", err)
		return
	}
	setupPath := filepath.Join(dir, "tuick-tack-setup.exe")

	d.ensureDownloader()
	ctx, cancel := d.updateContext()
	defer cancel()

	expected, err := d.downloader.ExpectedSHA256(ctx, asset, optionalAsset(checksums, hasChecksums))
	if err != nil {
		d.failInstall(downloadCode(err), err)
		return
	}
	d.updateProgress(updateKindSetup, updatePhaseDownloading, 0, asset.Size, "", "")
	if err := d.downloader.Download(ctx, asset.URL, setupPath, expected, asset.Size, func(received, total int64) {
		d.updateProgress(updateKindSetup, updatePhaseDownloading, received, total, "", "")
	}); err != nil {
		d.failInstall(downloadCode(err), err)
		return
	}
	d.updateProgress(updateKindSetup, updatePhaseVerifying, asset.Size, asset.Size, "", "")

	// A countdown can start from the tray while the setup downloads. Do not
	// discard it without the explicit confirmation the TUI collected.
	d.applyMu.Lock()
	active := d.timerActive()
	if active && !force {
		d.updating = false
		d.applyMu.Unlock()
		d.updateProgress(updateKindSetup, updatePhaseFailed, 0, 0, "", "timer_active")
		return
	}
	d.applyMu.Unlock()

	spawn := d.spawnUpdater
	if spawn == nil {
		spawn = updater.Spawn
	}
	if d.selfExe == "" {
		d.failInstall("write", errors.New("current executable is unavailable"))
		return
	}
	if err := spawn(setupPath, d.selfExe, d.pipe); err != nil {
		d.failInstall("setup_failed", err)
		return
	}
	d.logger.Printf("update: setup helper started for %s", info.Version)
	d.updateProgress(updateKindSetup, updatePhaseInstalling, asset.Size, asset.Size, "", "")
	d.QuitForUpdate()
}

// downloadPortable saves the new portable executable in Downloads. It never
// restarts the app and is safe while a countdown runs.
func (d *Daemon) downloadPortable(c *ipc.Session) {
	info, ok := d.cachedUpdate()
	if !ok || !info.CanDownloadPortable() {
		d.updateProgress(updateKindPortable, updatePhaseFailed, 0, 0, "", "unavailable")
		return
	}
	if buildinfo.Version == "dev" {
		d.updateProgress(updateKindPortable, updatePhaseFailed, 0, 0, "", "unsupported")
		return
	}

	d.portableMu.Lock()
	if d.portableActive {
		d.portableMu.Unlock()
		d.updateProgress(updateKindPortable, updatePhaseFailed, 0, 0, "", "busy")
		return
	}
	d.portableActive = true
	d.portableMu.Unlock()
	defer func() {
		d.portableMu.Lock()
		d.portableActive = false
		d.portableMu.Unlock()
	}()

	asset, _ := info.PortableAsset()
	checksums, hasChecksums := info.ChecksumsAsset()
	dir, err := downloadsDir()
	if err != nil {
		d.failPortable("write", err)
		return
	}
	name := "tuick-tack-" + sanitizeVersion(info.Version) + ".exe"
	dst := filepath.Join(dir, name)

	d.ensureDownloader()
	ctx, cancel := d.updateContext()
	defer cancel()

	expected, err := d.downloader.ExpectedSHA256(ctx, asset, optionalAsset(checksums, hasChecksums))
	if err != nil {
		d.failPortable(downloadCode(err), err)
		return
	}
	d.updateProgress(updateKindPortable, updatePhaseDownloading, 0, asset.Size, "", "")
	if err := d.downloader.Download(ctx, asset.URL, dst, expected, asset.Size, func(received, total int64) {
		d.updateProgress(updateKindPortable, updatePhaseDownloading, received, total, "", "")
	}); err != nil {
		d.failPortable(downloadCode(err), err)
		return
	}
	d.logger.Printf("update: portable %s saved to %s", info.Version, dst)
	d.updateProgress(updateKindPortable, updatePhaseDone, asset.Size, asset.Size, dst, "")
}

func (d *Daemon) ensureDownloader() {
	if d.downloader == nil {
		d.downloader = update.NewDownloader(buildinfo.Version)
	}
}

func (d *Daemon) updateDirectory() (string, error) {
	if d.updateDir != nil {
		return d.updateDir()
	}
	return updater.UpdateDir()
}

func (d *Daemon) updateContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	go func() {
		select {
		case <-d.quitCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (d *Daemon) updateProgress(kind, phase string, received, total int64, path, code string) {
	msg := protocol.NewUpdateProgress(protocol.UpdateProgressMessage{
		Kind:     kind,
		Phase:    phase,
		Received: received,
		Total:    total,
		Path:     path,
		Error:    code,
	})
	if d.progress != nil {
		d.progress(msg)
		return
	}
	if d.server == nil {
		return
	}
	d.server.Broadcast(msg)
}

func (d *Daemon) failInstall(code string, err error) {
	d.endApply()
	d.logger.Printf("update setup: %v", err)
	d.updateProgress(updateKindSetup, updatePhaseFailed, 0, 0, "", code)
}

func (d *Daemon) failPortable(code string, err error) {
	d.logger.Printf("update portable: %v", err)
	d.updateProgress(updateKindPortable, updatePhaseFailed, 0, 0, "", code)
}

func (d *Daemon) endApply() {
	d.applyMu.Lock()
	d.updating = false
	d.applyMu.Unlock()
}

// timerActive reports whether a countdown is running or paused. A finished
// countdown has no remaining state to lose.
func (d *Daemon) timerActive() bool {
	switch d.engine.Snapshot().Status {
	case engine.Running, engine.Paused:
		return true
	}
	return false
}

func optionalAsset(a update.Asset, ok bool) *update.Asset {
	if !ok {
		return nil
	}
	return &a
}

func downloadCode(err error) string {
	if errors.Is(err, update.ErrChecksum) {
		return "checksum"
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return "write"
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return "write"
	}
	return "network"
}

func sanitizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "update"
	}
	var b strings.Builder
	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "update"
	}
	return out
}
