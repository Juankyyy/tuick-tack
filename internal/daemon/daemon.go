// Package daemon wires the countdown engine, IPC server, tray and sound into
// the background process. The daemon is the single source of truth.
package daemon

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tray"
)

// Daemon owns the runtime state. Actions is implemented by this type.
type Daemon struct {
	logger  *log.Logger
	store   *config.Store
	engine  *engine.Engine
	server  *ipc.Server
	tray    *tray.Tray
	sound   *soundPlayer
	trayExe string

	mu        sync.RWMutex
	cfg       config.Config
	autostart bool

	quitCh   chan struct{}
	quitOnce sync.Once
	loopDone chan struct{}
}

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

	trayExe, _ := os.Executable()
	d := &Daemon{
		logger:   logger,
		store:    store,
		engine:   engine.New(engine.SystemClock()),
		sound:    player,
		trayExe:  trayExe,
		cfg:      cfg,
		quitCh:   make(chan struct{}),
		loopDone: make(chan struct{}),
	}
	d.autostart = AutostartEnabled(trayExe)
	d.tray = tray.New(d)

	server := ipc.NewServer(pipe, buildinfo.Version, d)
	d.server = server
	if err := server.Start(); err != nil {
		logger.Printf("ipc server: %v", err)
		return 1
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
	case protocol.TypeGetState:
		c.Send(protocol.NewState(d.Snapshot()))
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
	if d.SoundEnabled() {
		if err := d.sound.Play(); err != nil {
			d.logger.Printf("play chime: %v", err)
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

func (d *Daemon) commitConfig(c *ipc.Session, cfg config.Config) bool {
	if err := d.store.Save(cfg); err != nil {
		d.logger.Printf("save config: %v", err)
		c.Send(protocol.NewError(protocol.CodeConfigWrite, err.Error()))
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

// ToggleAutostart implements tray.Actions.
func (d *Daemon) ToggleAutostart() {
	next := !d.AutostartEnabled()
	if d.trayExe == "" {
		d.logger.Printf("autostart: executable path unavailable")
		return
	}
	if err := SetAutostart(d.trayExe, next); err != nil {
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
