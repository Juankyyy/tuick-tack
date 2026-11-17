// Package daemon wires the countdown engine, IPC server, tray and sound into
// the background process. The daemon is the single source of truth.
package daemon

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"tuick-tack/internal/browser"
	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/notify"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tray"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/update"
)

// Daemon owns the runtime state. Actions is implemented by this type.
type Daemon struct {
	logger  *log.Logger
	store   *config.Store
	engine  *engine.Engine
	server  *ipc.Server
	tray    *tray.Tray
	sound   *soundPlayer
	exePath string

	mu        sync.RWMutex
	cfg       config.Config
	autostart bool

	checker *update.Checker
	checkMu sync.Mutex

	// notifier is the native Windows toast used for manual checks started from
	// the tray. Tests inject a recorder; nil disables it.
	notifier func(title, message, launch string) error

	updateMu    sync.RWMutex
	updateInfo  update.Info
	updateOK    bool
	updateState updateCheckState

	quitCh   chan struct{}
	quitOnce sync.Once
	loopDone chan struct{}
}

// updateCheckState is the outcome of the last release check, shown by the tray
// and answered to a TUI that asked manually.
type updateCheckState string

const (
	updateCheckIdle     updateCheckState = "idle"
	updateCheckChecking updateCheckState = "checking"
	updateCheckUpToDate updateCheckState = "up_to_date"
	updateCheckFailed   updateCheckState = "failed"
)

// Run starts the daemon and blocks on the systray loop. It returns the process
// exit code: 0 when another instance already owns the mutex.
func Run() int {
	logger, closeLog, cache, logErr := SetupLogging()
	defer closeLog()
	if logErr != nil {
		logger.Printf("logging unavailable: %v", logErr)
	}
	LogStartup(logger)

	pipe, err := ipc.PipePath()
	if err != nil {
		logger.Printf("pipe path: %v", err)
		return 1
	}
	mutex, already, err := AcquireInstance(InstanceMutexName(pipe))
	if err != nil {
		logger.Printf("instance: %v", err)
		return 1
	}
	if already {
		logger.Println("another daemon is already running; exiting")
		return 0
	}
	defer windows.CloseHandle(mutex)
	logger.Printf("pipe: %s", pipe)

	path, err := config.DefaultPath()
	if err != nil {
		logger.Printf("config path: %v", err)
		return 1
	}
	store := config.NewStore(path)
	cfg, loadErr := store.Load()
	if loadErr != nil {
		logger.Printf("config: %v (using defaults)", loadErr)
	}
	if err := store.Save(cfg); err != nil {
		logger.Printf("save initial config: %v", err)
	}

	player, err := newSoundPlayer(cache)
	if err != nil {
		logger.Printf("sound unavailable: %v", err)
		player = &soundPlayer{}
	}

	exePath, err := AutostartExecutable()
	if err != nil {
		logger.Printf("autostart unavailable: %v", err)
		exePath = ""
	}
	if exePath != "" {
		migrated, err := MigrateAutostart(exePath)
		if err != nil {
			logger.Printf("autostart migration: %v", err)
		} else if migrated {
			logger.Printf("autostart: migrated to %s", exePath)
		}
	}
	d := &Daemon{
		logger:   logger,
		store:    store,
		engine:   engine.New(engine.SystemClock()),
		sound:    player,
		exePath:  exePath,
		checker:  update.NewChecker(buildinfo.Version),
		notifier: notify.Show,
		cfg:      cfg,
		quitCh:   make(chan struct{}),
		loopDone: make(chan struct{}),
	}
	d.autostart = AutostartEnabled(exePath)
	if err := d.sound.Prepare(cfg.SoundName, cfg.Volume); err != nil {
		logger.Printf("prepare sound: %v", err)
	}
	d.tray = tray.New(d)

	server := ipc.NewServer(pipe, buildinfo.Version, d)
	d.server = server
	if err := server.Start(); err != nil {
		logger.Printf("ipc server: %v", err)
		return 1
	}

	// Development builds never check on their own; the tray and the TUI can
	// still ask for a manual check.
	if buildinfo.Version != "dev" {
		go d.updateLoop()
	}

	go d.loop()
	d.tray.Run()

	// The systray loop can exit without going through Quit (session logoff).
	d.stopLoop()
	<-d.loopDone
	if err := server.Close(); err != nil {
		logger.Printf("close ipc server: %v", err)
	}
	d.sound.Stop()
	logger.Printf("stopped")
	return 0
}

func (d *Daemon) loop() {
	defer close(d.loopDone)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	lastTray := time.Now()
	for {
		select {
		case <-d.quitCh:
			return
		case now := <-ticker.C:
			st, crossed := d.engine.Tick()
			if crossed {
				d.onFinish(st)
			}
			if crossed || st.Status == engine.Running {
				d.server.BroadcastState(st)
			}
			if now.Sub(lastTray) >= time.Second {
				lastTray = now
				d.tray.Refresh()
			}
		}
	}
}

// Snapshot implements tray.Actions.
func (d *Daemon) Snapshot() engine.State { return d.engine.Snapshot() }

// Config implements tray.Actions and ipc.Backend.
func (d *Daemon) Config() config.Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg.Copy()
}

// SoundEnabled implements tray.Actions.
func (d *Daemon) SoundEnabled() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg.Sound
}

// AutostartEnabled implements tray.Actions.
func (d *Daemon) AutostartEnabled() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.autostart
}

// UpdateAvailable implements tray.Actions.
func (d *Daemon) UpdateAvailable() (latest, url string, ok bool) {
	d.updateMu.RLock()
	defer d.updateMu.RUnlock()
	if !d.updateOK {
		return "", "", false
	}
	return d.updateInfo.Version, d.updateInfo.URL, true
}

// UpdateCheck implements tray.Actions. It reports the last check as idle,
// checking, up_to_date or failed.
func (d *Daemon) UpdateCheck() (state, current string) {
	d.updateMu.RLock()
	s := d.updateState
	d.updateMu.RUnlock()
	if s == "" {
		s = updateCheckIdle
	}
	return string(s), buildinfo.Version
}

// Update implements ipc.Backend. The handshake carries the cached release so a
// TUI that connects after the check still sees the notice.
func (d *Daemon) Update() *protocol.UpdatePayload {
	latest, url, ok := d.UpdateAvailable()
	if !ok {
		return nil
	}
	return &protocol.UpdatePayload{Latest: latest, URL: url}
}

// OpenUpdate implements tray.Actions.
func (d *Daemon) OpenUpdate() {
	_, url, ok := d.UpdateAvailable()
	if !ok {
		return
	}
	if err := browser.Open(url); err != nil {
		d.logger.Printf("open release: %v", err)
	}
}

// CheckUpdates implements tray.Actions. The request runs in the background so
// the tray stays responsive, and it raises a native Windows notification with
// the result.
func (d *Daemon) CheckUpdates() {
	go d.checkUpdates(nil, true)
}

// checkUpdates performs a manual check and replies to the session that asked.
// It also refreshes the cached release and the tray, so the TUI and the tray
// share one result. notifyUser is true only for checks started from the tray.
func (d *Daemon) checkUpdates(c *ipc.Session, notifyUser bool) {
	d.setUpdateCheckState(updateCheckChecking)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	info, err := d.fetchLatest(ctx)
	cancel()
	if err != nil {
		d.logger.Printf("update check: %v", err)
		d.setUpdateCheckState(updateCheckFailed)
		if c != nil {
			c.Send(protocol.NewUpdateResult(protocol.UpdateResultMessage{
				Current: buildinfo.Version,
				Error:   "failed",
			}))
		}
		if notifyUser {
			d.notifyUpdate(update.Info{}, err)
		}
		return
	}

	if update.IsNewer(buildinfo.Version, info.Version) {
		d.setUpdate(info)
		d.setUpdateCheckState(updateCheckIdle)
	} else {
		d.clearUpdate()
		d.setUpdateCheckState(updateCheckUpToDate)
	}
	if c != nil {
		c.Send(protocol.NewUpdateResult(protocol.UpdateResultMessage{
			Current: buildinfo.Version,
			Latest:  info.Version,
			URL:     info.URL,
		}))
	}
	if notifyUser {
		d.notifyUpdate(info, nil)
	}
}

// notifyUpdate sends the result of a tray-initiated check as a Windows toast.
// The notification is best effort: a failure is logged and the tray label still
// carries the result.
func (d *Daemon) notifyUpdate(info update.Info, checkErr error) {
	if d.notifier == nil {
		return
	}
	var title, message, launch string
	switch {
	case checkErr != nil:
		title = copy.T(copy.NotifyFailedTitle)
		message = copy.T(copy.NotifyFailedBody)
	case update.IsNewer(buildinfo.Version, info.Version):
		title = copy.T(copy.NotifyFoundTitle)
		message = copy.T(copy.NotifyFoundBody, info.Version)
		launch = info.URL
	default:
		title = copy.T(copy.NotifyUpToDateTitle)
		message = copy.T(copy.NotifyUpToDateBody, buildinfo.Version)
	}
	if err := d.notifier(title, message, launch); err != nil {
		d.logger.Printf("windows notification: %v", err)
	}
}

// fetchLatest serializes manual and automatic checks so the app never talks to
// GitHub twice at once.
func (d *Daemon) fetchLatest(ctx context.Context) (update.Info, error) {
	d.checkMu.Lock()
	defer d.checkMu.Unlock()
	if d.checker == nil {
		d.checker = update.NewChecker(buildinfo.Version)
	}
	return d.checker.Latest(ctx)
}

// updateLoop checks GitHub once at startup and then every CheckInterval. It is
// only started by release builds; failures are silent and retried later.
func (d *Daemon) updateLoop() {
	delay := time.Duration(0)
	for {
		timer := time.NewTimer(delay)
		select {
		case <-d.quitCh:
			timer.Stop()
			return
		case <-timer.C:
		}
		d.setUpdateCheckState(updateCheckChecking)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		info, err := d.fetchLatest(ctx)
		cancel()
		if err != nil {
			d.logger.Printf("update check: %v", err)
			d.setUpdateCheckState(updateCheckFailed)
			delay = update.Delay(true)
			continue
		}
		if update.IsNewer(buildinfo.Version, info.Version) {
			d.setUpdate(info)
			d.setUpdateCheckState(updateCheckIdle)
		} else {
			d.clearUpdate()
			d.setUpdateCheckState(updateCheckUpToDate)
		}
		delay = update.Delay(false)
	}
}

// setUpdate caches a newer release and tells the TUI and the tray once.
func (d *Daemon) setUpdate(info update.Info) {
	d.updateMu.Lock()
	changed := !d.updateOK || d.updateInfo != info
	d.updateInfo = info
	d.updateOK = true
	d.updateMu.Unlock()
	if !changed {
		return
	}
	d.logger.Printf("update available: %s (current %s) %s",
		info.Version, buildinfo.Version, info.URL)
	if d.server != nil {
		d.server.Broadcast(protocol.NewUpdate(protocol.UpdatePayload{
			Latest: info.Version,
			URL:    info.URL,
		}))
	}
	d.tray.Refresh()
}

// clearUpdate drops a stale notice after a check that found no newer release.
func (d *Daemon) clearUpdate() {
	d.updateMu.Lock()
	changed := d.updateOK
	d.updateInfo = update.Info{}
	d.updateOK = false
	d.updateMu.Unlock()
	if changed {
		d.tray.Refresh()
	}
}

func (d *Daemon) setUpdateCheckState(state updateCheckState) {
	d.updateMu.Lock()
	changed := d.updateState != state
	d.updateState = state
	d.updateMu.Unlock()
	if changed {
		d.tray.Refresh()
	}
}

// Command implements ipc.Backend. Messages arrive validated by the protocol
// layer; preset limits and IDs are checked here.
func (d *Daemon) Command(c *ipc.Session, msg protocol.ClientMessage) {
	switch msg.Type {
	case protocol.TypeStart:
		d.startTimer(msg.Seconds, msg.Label, msg.PresetID)
	case protocol.TypePause:
		before := d.Snapshot().Status
		if d.engine.Pause() {
			d.publish(d.Snapshot(), before)
		}
	case protocol.TypeResume:
		before := d.Snapshot().Status
		if d.engine.Resume() {
			d.publish(d.Snapshot(), before)
		}
	case protocol.TypeStop:
		d.sound.Stop()
		d.engine.Stop()
		d.server.BroadcastState(d.Snapshot())
		d.tray.Refresh()
	case protocol.TypeDismiss:
		d.sound.Stop()
		if d.engine.Dismiss() {
			d.server.BroadcastState(d.Snapshot())
			d.tray.Refresh()
		}
	case protocol.TypeRestart:
		d.sound.Stop()
		before := d.Snapshot().Status
		if d.engine.Restart() {
			d.publish(d.Snapshot(), before)
		} else {
			c.Send(protocol.NewError(protocol.CodeInvalidCommand, "nothing to repeat"))
		}
	case protocol.TypeAdjust:
		before := d.Snapshot().Status
		if d.engine.Adjust(time.Duration(msg.DeltaSeconds) * time.Second) {
			d.publish(d.Snapshot(), before)
		}
	case protocol.TypePresetAdd:
		d.addPreset(c, msg.Label, msg.Seconds)
	case protocol.TypePresetUpdate:
		d.updatePreset(c, msg.ID, msg.Label, msg.Seconds)
	case protocol.TypePresetDelete:
		d.deletePreset(c, msg.ID)
	case protocol.TypePresetsRestoreDefault:
		d.restorePresets(c)
	case protocol.TypeSetTheme:
		d.setTheme(c, msg.Name)
	case protocol.TypeSetSound:
		if msg.Enabled != nil {
			if err := d.changeSound(*msg.Enabled); err != nil {
				c.Send(protocol.NewError(protocol.CodeConfigWrite, err.Error()))
			}
		}
	case protocol.TypeSetSoundName:
		d.setSoundName(c, msg.SoundName)
	case protocol.TypeSetVolume:
		if msg.Volume != nil {
			d.setVolume(c, *msg.Volume)
		}
	case protocol.TypePreviewSound:
		d.previewSound()
	case protocol.TypeStopSound:
		d.sound.Stop()
	case protocol.TypeGetState:
		c.Send(protocol.NewState(d.Snapshot()))
	case protocol.TypeCheckUpdate:
		go d.checkUpdates(c, false)
	case protocol.TypeQuitDaemon:
		go d.Quit()
	default:
		c.Send(protocol.NewError(protocol.CodeInvalidCommand, "unknown command"))
	}
}

func (d *Daemon) startTimer(seconds int, label, presetID string) {
	d.sound.Stop()
	before := d.Snapshot().Status
	d.engine.Start(time.Duration(seconds)*time.Second, strings.TrimSpace(label), presetID)
	d.publish(d.Snapshot(), before)
}

func (d *Daemon) publish(st engine.State, before engine.Status) {
	if st.Status == engine.Finished && before != engine.Finished {
		d.onFinish(st)
	}
	d.server.BroadcastState(st)
	d.tray.Refresh()
}

func (d *Daemon) onFinish(st engine.State) {
	d.stopSound()
	cfg := d.Config()
	if cfg.Sound && cfg.Volume > 0 {
		if err := d.sound.Play(cfg.SoundName, cfg.Volume); err != nil {
			d.logger.Printf("play sound %q at %d%%: %v", cfg.SoundName, cfg.Volume, err)
		}
	}
	d.logger.Printf("countdown finished: %q (%s)", st.Label, time.Duration(st.Duration))
}

func (d *Daemon) stopSound() {
	d.sound.Stop()
}

func (d *Daemon) addPreset(c *ipc.Session, label string, seconds int) {
	cfg := d.Config()
	if len(cfg.Presets) >= config.MaxPresets {
		c.Send(protocol.NewError(protocol.CodePresetLimit, "maximum of 16 presets reached"))
		return
	}
	cfg.Presets = append(cfg.Presets, config.NewPreset(label, seconds))
	d.commitConfig(c, cfg)
}

func (d *Daemon) updatePreset(c *ipc.Session, id, label string, seconds int) {
	cfg := d.Config()
	for i := range cfg.Presets {
		if cfg.Presets[i].ID == id {
			cfg.Presets[i].Label = strings.TrimSpace(label)
			cfg.Presets[i].Seconds = seconds
			d.commitConfig(c, cfg)
			return
		}
	}
	c.Send(protocol.NewError(protocol.CodePresetNotFound, "preset not found"))
}

func (d *Daemon) deletePreset(c *ipc.Session, id string) {
	cfg := d.Config()
	for i := range cfg.Presets {
		if cfg.Presets[i].ID == id {
			cfg.Presets = append(cfg.Presets[:i], cfg.Presets[i+1:]...)
			d.commitConfig(c, cfg)
			return
		}
	}
	c.Send(protocol.NewError(protocol.CodePresetNotFound, "preset not found"))
}

func (d *Daemon) restorePresets(c *ipc.Session) {
	cfg := d.Config()
	cfg.Presets = config.Default().Presets
	d.commitConfig(c, cfg)
}

func (d *Daemon) setTheme(c *ipc.Session, name string) {
	cfg := d.Config()
	cfg.Theme = name
	d.commitConfig(c, cfg)
}

func (d *Daemon) setSoundName(c *ipc.Session, name string) {
	if !config.ValidSound(name) {
		if c != nil {
			c.Send(protocol.NewError(protocol.CodeInvalidSound, "unknown sound"))
		}
		return
	}
	cfg := d.Config()
	if cfg.SoundName == name {
		return
	}
	cfg.SoundName = name
	if !d.commitConfig(c, cfg) {
		return
	}
	d.sound.Stop()
	if err := d.sound.Prepare(name, cfg.Volume); err != nil {
		d.logger.Printf("prepare sound %q at %d%%: %v", name, cfg.Volume, err)
		return
	}
	if cfg.Sound && cfg.Volume > 0 {
		if err := d.sound.Play(name, cfg.Volume); err != nil {
			d.logger.Printf("preview sound %q: %v", name, err)
		}
	}
}

func (d *Daemon) setVolume(c *ipc.Session, volume int) {
	if !config.ValidVolume(volume) {
		if c != nil {
			c.Send(protocol.NewError(protocol.CodeInvalidVolume, "volume out of range"))
		}
		return
	}
	cfg := d.Config()
	if cfg.Volume == volume {
		return
	}
	cfg.Volume = volume
	if !d.commitConfig(c, cfg) {
		return
	}
	d.sound.Stop()
	if err := d.sound.Prepare(cfg.SoundName, volume); err != nil {
		d.logger.Printf("prepare sound %q at %d%%: %v", cfg.SoundName, volume, err)
		return
	}
	if cfg.Sound && volume > 0 {
		if err := d.sound.Play(cfg.SoundName, volume); err != nil {
			d.logger.Printf("preview sound %q: %v", cfg.SoundName, err)
		}
	}
}

// previewSound plays the current selection even when Sound is off. The tray and
// TUI use it as an explicit "hear this" action; the finish path still respects
// the Sound toggle.
func (d *Daemon) previewSound() {
	cfg := d.Config()
	if cfg.Volume <= 0 {
		d.sound.Stop()
		return
	}
	if err := d.sound.Play(cfg.SoundName, cfg.Volume); err != nil {
		d.logger.Printf("preview sound %q at %d%%: %v", cfg.SoundName, cfg.Volume, err)
	}
}

func (d *Daemon) commitConfig(c *ipc.Session, cfg config.Config) bool {
	if err := d.store.Save(cfg); err != nil {
		d.logger.Printf("save config: %v", err)
		if c != nil {
			c.Send(protocol.NewError(protocol.CodeConfigWrite, err.Error()))
		}
		return false
	}
	d.mu.Lock()
	d.cfg = cfg
	d.mu.Unlock()
	d.server.BroadcastConfig(cfg)
	d.tray.Refresh()
	return true
}

func (d *Daemon) changeSound(enabled bool) error {
	cfg := d.Config()
	if cfg.Sound == enabled {
		return nil
	}
	cfg.Sound = enabled
	if err := d.store.Save(cfg); err != nil {
		return err
	}
	d.mu.Lock()
	d.cfg = cfg
	d.mu.Unlock()
	if !enabled {
		d.sound.Stop()
	}
	d.server.BroadcastConfig(cfg)
	d.tray.Refresh()
	return nil
}

// OpenTUI implements tray.Actions.
func (d *Daemon) OpenTUI() {
	if err := OpenTUI(); err != nil {
		d.logger.Printf("open TUI: %v", err)
	}
}

// StartPreset implements tray.Actions.
func (d *Daemon) StartPreset(p config.Preset) {
	if !config.ValidSeconds(p.Seconds) || !config.ValidLabel(p.Label) {
		d.logger.Printf("tray: invalid preset %+v", p)
		return
	}
	d.startTimer(p.Seconds, p.Label, p.ID)
}

// StartCustom implements tray.Actions. The TUI opens ready to type.
func (d *Daemon) StartCustom() {
	d.OpenTUI()
}

// TogglePause implements tray.Actions.
func (d *Daemon) TogglePause() {
	before := d.Snapshot().Status
	switch before {
	case engine.Running:
		if d.engine.Pause() {
			d.publish(d.Snapshot(), before)
		}
	case engine.Paused:
		if d.engine.Resume() {
			d.publish(d.Snapshot(), before)
		}
	}
}

// Stop implements tray.Actions.
func (d *Daemon) Stop() {
	d.sound.Stop()
	d.engine.Stop()
	d.server.BroadcastState(d.Snapshot())
	d.tray.Refresh()
}

// ToggleSound implements tray.Actions.
func (d *Daemon) ToggleSound() {
	if err := d.changeSound(!d.SoundEnabled()); err != nil {
		d.logger.Printf("toggle sound: %v", err)
	}
}

// SetSoundName implements tray.Actions.
func (d *Daemon) SetSoundName(name string) {
	d.setSoundName(nil, name)
}

// SetVolume implements tray.Actions.
func (d *Daemon) SetVolume(volume int) {
	d.setVolume(nil, volume)
}

// PreviewSound implements tray.Actions.
func (d *Daemon) PreviewSound() {
	d.previewSound()
}

// ToggleAutostart implements tray.Actions.
func (d *Daemon) ToggleAutostart() {
	next := !d.AutostartEnabled()
	if d.exePath == "" {
		d.logger.Printf("autostart: executable path unavailable")
		return
	}
	if err := SetAutostart(d.exePath, next); err != nil {
		d.logger.Printf("autostart: %v", err)
		return
	}
	d.mu.Lock()
	d.autostart = next
	d.mu.Unlock()
	d.tray.Refresh()
}

// Quit stops the loop and the systray. It is safe to call more than once.
func (d *Daemon) Quit() {
	d.quitOnce.Do(func() {
		close(d.quitCh)
		if d.tray != nil {
			d.tray.Quit()
		}
	})
}

func (d *Daemon) stopLoop() {
	d.quitOnce.Do(func() { close(d.quitCh) })
}
