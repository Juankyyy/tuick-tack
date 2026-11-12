package tui

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/config"
	"tuick-tack/internal/daemon"
	"tuick-tack/internal/duration"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/fx"
	"tuick-tack/internal/tui/theme"
)

type modalKind int

const (
	modalNone modalKind = iota
	modalHelp
	modalConfirmStop
	modalConfirmDelete
	modalPresets
	modalPresetForm
	modalSound
)

type formMode int

const (
	formAdd formMode = iota
	formEdit
)

type formState struct {
	mode     formMode
	id       string
	label    string
	duration string
	focus    int
	err      string
}

type hitAction int

const (
	hitNone hitAction = iota
	hitChip
	hitPresetRow
	hitPlus
	hitPause
	hitAdjustPlus
	hitAdjustMinus
	hitCancel
	hitClose
	hitConfirmYes
	hitConfirmNo
	hitPresetAdd
	hitPresetEdit
	hitPresetDelete
	hitPresetRestore
	hitPresetStart
	hitFormName
	hitFormDuration
	hitFormSave
	hitFormCancel
	hitRepeat
	hitDismiss
	hitSoundRow
	hitSoundBar
	hitSoundVolumeDown
	hitSoundVolumeUp
	hitSoundPreview
	hitSoundMute
	hitSoundClose
)

// presetFocusList means the preset list owns the keyboard focus; any other
// value is an index into presetActionDefs.
const presetFocusList = -1

// presetActionDef describes one button in the presets modal. The view and the
// keyboard handler share the list so both stay in sync.
type presetActionDef struct {
	label       copy.Key
	action      hitAction
	needsPreset bool
}

var presetActionDefs = []presetActionDef{
	{copy.Add, hitPresetAdd, false},
	{copy.Edit, hitPresetEdit, true},
	{copy.Delete, hitPresetDelete, true},
	{copy.Restore, hitPresetRestore, false},
	{copy.Start, hitPresetStart, true},
	{copy.Close, hitClose, false},
}

type hitbox struct {
	x, y, w, h int
	action     hitAction
	index      int
}

type connectedMsg struct {
	client *ipc.Client
	hello  protocol.HelloMessage
	err    error
}

type serverEventMsg struct {
	msg protocol.ServerMessage
	err error
}

type sentMsg struct{ err error }

type tickMsg time.Time

// Model is the Bubble Tea model. All timer truth comes from the daemon; the
// model only interpolates between snapshots.
type Model struct {
	pipe string
	now  func() time.Time
	seed int64

	client       *ipc.Client
	connected    bool
	connecting   bool
	reconnecting bool

	state    protocol.StateMessage
	hasState bool
	snapAt   time.Time
	cfg      config.Config
	pal      theme.Theme

	width, height int
	hits          []hitbox

	input    string
	inputErr string
	selected int

	modal        modalKind
	presetCursor int
	presetScroll int
	presetFocus  int
	soundCursor  int
	form         formState
	deleteTarget config.Preset

	finishSeen       int64
	celebrating      bool
	celebrationStart time.Time
	finishedAt       time.Time
	confetti         *fx.Confetti
	dust             *fx.Dust
	animTime         float64
	lastTick         time.Time

	toast      string
	toastUntil time.Time
}

// New creates the TUI model.
func New(pipe string) *Model {
	return newModel(pipe, time.Now, time.Now().UnixNano())
}

func newModel(pipe string, now func() time.Time, seed int64) *Model {
	if now == nil {
		now = time.Now
	}
	pal := theme.Get("midnight")
	cfg := config.Default()
	cfg.Theme = "midnight"
	return &Model{
		pipe:        pipe,
		now:         now,
		seed:        seed,
		cfg:         cfg,
		pal:         pal,
		width:       80,
		height:      24,
		presetFocus: presetFocusList,
		connecting:  true,
		dust:        fx.NewDust(seed, 80, 24),
		lastTick:    now(),
	}
}

// Init starts the connection and the render tick.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(connectCmd(m.pipe), tickCmd(100*time.Millisecond))
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.dust != nil {
			m.dust.Resize(msg.Width, msg.Height)
		}
		if m.confetti != nil {
			m.confetti.Resize(msg.Width, msg.Height)
		}
		return m, nil
	case tickMsg:
		return m, m.handleTick()
	case connectedMsg:
		return m, m.handleConnected(msg)
	case serverEventMsg:
		return m, m.handleServerEvent(msg)
	case sentMsg:
		if msg.err != nil {
			m.showToast(copy.T(copy.DaemonLost))
		}
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case tea.MouseClickMsg:
		return m, m.handleMouse(msg)
	}
	return m, nil
}

func connectCmd(pipe string) tea.Cmd {
	return func() tea.Msg {
		c, hello, err := daemon.Ensure(pipe)
		return connectedMsg{client: c, hello: hello, err: err}
	}
}

func reconnectCmd(pipe string) tea.Cmd {
	return func() tea.Msg {
		for {
			c, hello, err := daemon.Ensure(pipe)
			if err == nil {
				return connectedMsg{client: c, hello: hello}
			}
			time.Sleep(time.Second)
		}
	}
}

func readCmd(c *ipc.Client) tea.Cmd {
	return func() tea.Msg {
		msg, err := c.ReadMessage()
		return serverEventMsg{msg: msg, err: err}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) handleConnected(msg connectedMsg) tea.Cmd {
	if msg.err != nil {
		m.connected = false
		m.connecting = false
		if m.client != nil {
			_ = m.client.Close()
			m.client = nil
		}
		m.showToast(copy.T(copy.Disconnected))
		if m.reconnecting {
			return nil
		}
		m.reconnecting = true
		return reconnectCmd(m.pipe)
	}
	m.client = msg.client
	m.connected = true
	m.connecting = false
	m.reconnecting = false
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: msg.hello.Config})
	return readCmd(m.client)
}

func (m *Model) handleServerEvent(msg serverEventMsg) tea.Cmd {
	if msg.err != nil {
		if m.client != nil {
			_ = m.client.Close()
			m.client = nil
		}
		m.connected = false
		m.connecting = false
		m.showToast(copy.T(copy.DaemonLost))
		if m.reconnecting {
			return nil
		}
		m.reconnecting = true
		return reconnectCmd(m.pipe)
	}
	switch ev := msg.msg.(type) {
	case protocol.StateMessage:
		m.applyState(ev)
	case protocol.ConfigMessage:
		m.applyConfig(ev)
	case protocol.ErrorMessage:
		m.showToast(ev.Message)
	}
	if m.client != nil {
		return readCmd(m.client)
	}
	return nil
}

func (m *Model) handleTick() tea.Cmd {
	now := m.now()
	dt := now.Sub(m.lastTick)
	if dt < 0 || dt > time.Second {
		dt = 100 * time.Millisecond
	}
	m.lastTick = now
	m.animTime += dt.Seconds()
	if m.dust != nil {
		m.dust.Update(dt.Seconds())
	}
	if m.celebrating && m.confetti != nil {
		m.confetti.Update(dt.Seconds())
	}
	if m.toast != "" && now.After(m.toastUntil) {
		m.toast = ""
	}
	if m.celebrating && now.Sub(m.celebrationStart) > 6*time.Second {
		m.confetti = nil
	}
	if m.celebrating && now.Sub(m.celebrationStart) < 6*time.Second {
		return tickCmd(33 * time.Millisecond)
	}
	return tickCmd(100 * time.Millisecond)
}

func (m *Model) applyConfig(cfg protocol.ConfigMessage) {
	c := config.Config{
		Version:   config.Version,
		Theme:     cfg.Theme,
		Sound:     cfg.Sound,
		SoundName: cfg.SoundName,
		Volume:    cfg.Volume,
		Presets:   append([]config.Preset(nil), cfg.Presets...),
	}
	if !config.ValidTheme(c.Theme) {
		c.Theme = "midnight"
	}
	if c.SoundName == "" {
		// An older daemon does not send the sound fields at all.
		c.SoundName = config.DefaultSound
		c.Volume = config.DefaultVolume
	} else if !config.ValidSound(c.SoundName) {
		c.SoundName = config.DefaultSound
	}
	if !config.ValidVolume(c.Volume) {
		c.Volume = config.DefaultVolume
	}
	m.cfg = c
	m.pal = theme.Get(c.Theme)
	if m.modal == modalSound {
		m.soundCursor = soundIndex(c.SoundName)
	}
	if m.selected >= len(c.Presets) {
		m.selected = maxInt(0, len(c.Presets)-1)
	}
	if m.presetCursor >= len(c.Presets) {
		m.presetCursor = maxInt(0, len(c.Presets)-1)
	}
	m.clampPresetFocus()
	if m.form.mode == formEdit {
		found := false
		for _, p := range c.Presets {
			if p.ID == m.form.id {
				found = true
				break
			}
		}
		if !found {
			m.modal = modalNone
		}
	}
}

func (m *Model) applyState(st protocol.StateMessage) {
	first := !m.hasState
	prev := m.state
	m.state = st
	m.hasState = true
	m.snapAt = m.now()

	switch {
	case st.Status == string(engine.Finished) && st.FinishSeq != m.finishSeen:
		m.finishSeen = st.FinishSeq
		m.startCelebration(first)
	case first && st.Status == string(engine.Finished):
		m.startCelebration(true)
	case st.FinishSeq > m.finishSeen:
		// The TUI missed the finish because it was closed; do not celebrate a
		// timer that has already been restarted.
		m.finishSeen = st.FinishSeq
	}

	if st.Status != string(engine.Finished) {
		m.celebrating = false
		m.confetti = nil
	} else if m.modal != modalNone {
		m.modal = modalNone
		m.form.err = ""
	}

	if !first && prev.Status == string(engine.Running) && st.Status == string(engine.Running) &&
		st.FinishSeq == prev.FinishSeq &&
		(st.DurationMS != prev.DurationMS || st.Label != prev.Label) {
		m.showToast(copy.T(copy.ToastReplaced))
	}
}

func (m *Model) startCelebration(short bool) {
	m.celebrating = true
	m.celebrationStart = m.now()
	m.finishedAt = m.now()
	m.modal = modalNone
	m.form.err = ""
	m.input = ""
	m.inputErr = ""
	w, h := m.viewSize()
	seed := m.seed + m.state.FinishSeq*7919
	m.confetti = fx.NewCelebration(seed, w, h, m.pal.Confetti, m.pal.BG, short)
	m.dust = fx.NewDust(seed, w, h)
}

func (m *Model) showToast(s string) {
	m.toast = s
	m.toastUntil = m.now().Add(3 * time.Second)
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Key()
	if m.modal != modalNone {
		return m.handleModalKey(key)
	}
	text := key.Text
	status := m.status()

	if text == "q" {
		m.Close()
		return tea.Quit
	}
	switch status {
	case engine.Running, engine.Paused:
		return m.handleTimerKey(key)
	case engine.Finished:
		return m.handleFinishedKey(key)
	default:
		return m.handleIdleKey(key)
	}
}

func (m *Model) handleIdleKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeyEscape:
		m.input = ""
		m.inputErr = ""
		return nil
	case key.Code == tea.KeyBackspace:
		m.input = dropLastRune(m.input)
		m.inputErr = ""
		return nil
	case key.Code == tea.KeyEnter:
		return m.startTypedOrSelected()
	case key.Code == tea.KeyLeft:
		if len(m.cfg.Presets) > 0 {
			m.selected = (m.selected - 1 + len(m.cfg.Presets)) % len(m.cfg.Presets)
		}
		return nil
	case key.Code == tea.KeyRight:
		if len(m.cfg.Presets) > 0 {
			m.selected = (m.selected + 1) % len(m.cfg.Presets)
		}
		return nil
	case text == "?":
		m.modal = modalHelp
		return nil
	case text == "v":
		m.openSound()
		return nil
	case text == "p":
		m.openPresets()
		return nil
	case text == "t":
		return m.cycleTheme()
	case text == "S":
		return m.toggleSound()
	case text == "s":
		if m.acceptsUnit() {
			m.appendInput("s")
			return nil
		}
		return m.toggleSound()
	case isDigitText(text) || text == ":" || text == "h" || text == "m" || text == "s":
		m.appendInput(text)
		return nil
	}
	return nil
}

func (m *Model) handleTimerKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeySpace || text == " ":
		return m.togglePause()
	case key.Code == tea.KeyEscape:
		m.modal = modalConfirmStop
		return nil
	case text == "+" || text == "=":
		return m.adjust(60)
	case text == "-":
		return m.adjust(-60)
	case text == "?":
		m.modal = modalHelp
		return nil
	case text == "v":
		m.openSound()
		return nil
	}
	return nil
}

func (m *Model) handleFinishedKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeyEnter || key.Code == tea.KeyEscape:
		m.celebrating = false
		m.confetti = nil
		return m.send(protocol.ClientMessage{Type: protocol.TypeDismiss})
	case text == "r":
		m.celebrating = false
		m.confetti = nil
		return m.send(protocol.ClientMessage{Type: protocol.TypeRestart})
	case text == "?":
		m.modal = modalHelp
		return nil
	case text == "v":
		m.openSound()
		return nil
	}
	return nil
}

func (m *Model) handleModalKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch m.modal {
	case modalHelp:
		if key.Code == tea.KeyEscape || key.Code == tea.KeyEnter || text == "?" {
			m.modal = modalNone
		}
		return nil
	case modalConfirmStop:
		switch {
		case key.Code == tea.KeyEnter:
			m.modal = modalNone
			return m.send(protocol.ClientMessage{Type: protocol.TypeStop})
		case key.Code == tea.KeyEscape:
			m.modal = modalNone
		}
		return nil
	case modalConfirmDelete:
		switch {
		case key.Code == tea.KeyEnter:
			return m.confirmDelete()
		case key.Code == tea.KeyEscape:
			m.modal = modalPresets
		}
		return nil
	case modalPresets:
		return m.handlePresetListKey(key)
	case modalPresetForm:
		return m.handleFormKey(key)
	case modalSound:
		return m.handleSoundKey(key)
	}
	return nil
}

func (m *Model) handlePresetListKey(key tea.Key) tea.Cmd {
	text := key.Text
	n := len(m.cfg.Presets)
	m.clampPresetFocus()
	switch {
	case key.Code == tea.KeyEscape:
		m.modal = modalNone
	case key.Code == tea.KeyTab:
		// Tab moves between the list and the action bar; the bar is entered
		// at the first action that is available.
		if m.presetFocus == presetFocusList {
			m.presetFocus = m.firstPresetAction()
		} else {
			m.presetFocus = presetFocusList
		}
	case key.Code == tea.KeyLeft:
		if m.presetFocus != presetFocusList {
			m.movePresetFocus(-1)
		}
	case key.Code == tea.KeyRight:
		if m.presetFocus != presetFocusList {
			m.movePresetFocus(1)
		}
	case key.Code == tea.KeyUp:
		m.presetFocus = presetFocusList
		if n > 0 {
			m.presetCursor = (m.presetCursor - 1 + n) % n
		}
	case key.Code == tea.KeyDown:
		m.presetFocus = presetFocusList
		if n > 0 {
			m.presetCursor = (m.presetCursor + 1) % n
		}
	case key.Code == tea.KeyEnter:
		if m.presetFocus != presetFocusList {
			return m.activatePresetAction(m.presetFocus)
		}
		if p, ok := m.cursorPreset(); ok {
			m.selected = m.presetCursor
			m.modal = modalNone
			return m.startPreset(p)
		}
	case text == "a":
		return m.activatePresetAction(presetActionIndex(hitPresetAdd))
	case text == "e":
		return m.activatePresetAction(presetActionIndex(hitPresetEdit))
	case text == "d":
		return m.activatePresetAction(presetActionIndex(hitPresetDelete))
	case text == "r":
		return m.activatePresetAction(presetActionIndex(hitPresetRestore))
	}
	m.clampPresetCursor()
	return nil
}

func presetActionIndex(action hitAction) int {
	for i, def := range presetActionDefs {
		if def.action == action {
			return i
		}
	}
	return presetFocusList
}

func (m *Model) presetActionEnabled(i int) bool {
	if i < 0 || i >= len(presetActionDefs) {
		return false
	}
	return !presetActionDefs[i].needsPreset || len(m.cfg.Presets) > 0
}

func (m *Model) firstPresetAction() int {
	for i := 0; i < len(presetActionDefs); i++ {
		if m.presetActionEnabled(i) {
			return i
		}
	}
	return presetFocusList
}

// clampPresetFocus keeps the focus on an available action. When a preset is
// deleted while edit/delete/start is focused, focus moves to the next action.
func (m *Model) clampPresetFocus() {
	if m.presetFocus == presetFocusList || m.presetActionEnabled(m.presetFocus) {
		return
	}
	m.movePresetFocus(1)
}

func (m *Model) movePresetFocus(delta int) {
	if m.presetFocus == presetFocusList {
		m.presetFocus = m.firstPresetAction()
		return
	}
	for step := 1; step <= len(presetActionDefs); step++ {
		next := (m.presetFocus + delta*step) % len(presetActionDefs)
		if next < 0 {
			next += len(presetActionDefs)
		}
		if m.presetActionEnabled(next) {
			m.presetFocus = next
			return
		}
	}
}

// activatePresetAction routes keyboard activation through the same handler as
// a mouse click, so both input paths behave identically.
func (m *Model) activatePresetAction(i int) tea.Cmd {
	if !m.presetActionEnabled(i) {
		return nil
	}
	return m.performHit(hitbox{action: presetActionDefs[i].action})
}

func (m *Model) handleFormKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch key.Code {
	case tea.KeyEscape:
		m.modal = modalPresets
		m.form.err = ""
		return nil
	case tea.KeyTab:
		m.form.focus = (m.form.focus + 1) % 2
		return nil
	case tea.KeyEnter:
		return m.saveForm()
	case tea.KeyBackspace:
		if m.form.focus == 0 {
			m.form.label = dropLastRune(m.form.label)
		} else {
			m.form.duration = dropLastRune(m.form.duration)
		}
		m.form.err = ""
		return nil
	}
	if text == "" || len([]rune(text)) != 1 {
		return nil
	}
	r := []rune(text)[0]
	if unicode.IsControl(r) {
		return nil
	}
	if m.form.focus == 0 {
		if len([]rune(m.form.label)) < config.MaxLabelRunes {
			m.form.label += text
			m.form.err = ""
		}
		return nil
	}
	if len(m.form.duration) < 16 && (isDigitRune(r) || r == ':' || r == 'h' || r == 'm' || r == 's') {
		m.form.duration += string(r)
		m.form.err = ""
	}
	return nil
}

func (m *Model) handleSoundKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeyEscape || key.Code == tea.KeyEnter:
		return m.closeSound()
	case key.Code == tea.KeyUp:
		m.soundCursor = (m.soundCursor - 1 + len(config.Sounds)) % len(config.Sounds)
		return m.selectSound(config.Sounds[m.soundCursor])
	case key.Code == tea.KeyDown:
		m.soundCursor = (m.soundCursor + 1) % len(config.Sounds)
		return m.selectSound(config.Sounds[m.soundCursor])
	case key.Code == tea.KeyLeft || text == "-":
		return m.setVolume(m.cfg.Volume - 5)
	case key.Code == tea.KeyRight || text == "+" || text == "=":
		return m.setVolume(m.cfg.Volume + 5)
	case key.Code == tea.KeySpace || text == " ":
		return m.previewSound()
	case text == "m" || text == "M":
		return m.toggleSound()
	}
	return nil
}

func (m *Model) handleMouse(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button != tea.MouseLeft {
		return nil
	}
	for i := len(m.hits) - 1; i >= 0; i-- {
		hit := m.hits[i]
		if msg.X < hit.x || msg.X >= hit.x+hit.w || msg.Y < hit.y || msg.Y >= hit.y+hit.h {
			continue
		}
		return m.performHit(hit)
	}
	return nil
}

func (m *Model) performHit(hit hitbox) tea.Cmd {
	switch hit.action {
	case hitChip:
		if hit.index >= 0 && hit.index < len(m.cfg.Presets) {
			m.selected = hit.index
			return m.startPreset(m.cfg.Presets[hit.index])
		}
	case hitPlus:
		m.openForm(formAdd, config.Preset{})
	case hitPresetRow:
		m.presetCursor = hit.index
		m.presetFocus = presetFocusList
		m.clampPresetCursor()
	case hitPause:
		return m.togglePause()
	case hitAdjustPlus:
		return m.adjust(60)
	case hitAdjustMinus:
		return m.adjust(-60)
	case hitCancel:
		m.modal = modalConfirmStop
	case hitClose:
		m.closeModal()
	case hitConfirmYes:
		if m.modal == modalConfirmDelete {
			return m.confirmDelete()
		}
		m.modal = modalNone
		return m.send(protocol.ClientMessage{Type: protocol.TypeStop})
	case hitConfirmNo:
		if m.modal == modalConfirmDelete {
			m.modal = modalPresets
		} else {
			m.modal = modalNone
		}
	case hitPresetAdd:
		m.openForm(formAdd, config.Preset{})
	case hitPresetEdit:
		if p, ok := m.cursorPreset(); ok {
			m.openForm(formEdit, p)
		}
	case hitPresetDelete:
		if p, ok := m.cursorPreset(); ok {
			m.deleteTarget = p
			m.modal = modalConfirmDelete
		}
	case hitPresetRestore:
		m.showToast(copy.T(copy.ToastRestored))
		return m.send(protocol.ClientMessage{Type: protocol.TypePresetsRestoreDefault})
	case hitPresetStart:
		if p, ok := m.cursorPreset(); ok {
			m.modal = modalNone
			return m.startPreset(p)
		}
	case hitFormName:
		m.form.focus = 0
	case hitFormDuration:
		m.form.focus = 1
	case hitFormSave:
		return m.saveForm()
	case hitFormCancel:
		m.modal = modalPresets
		m.form.err = ""
	case hitRepeat:
		m.celebrating = false
		m.confetti = nil
		return m.send(protocol.ClientMessage{Type: protocol.TypeRestart})
	case hitDismiss:
		m.celebrating = false
		m.confetti = nil
		return m.send(protocol.ClientMessage{Type: protocol.TypeDismiss})
	case hitSoundRow:
		if hit.index >= 0 && hit.index < len(config.Sounds) {
			m.soundCursor = hit.index
			return m.selectSound(config.Sounds[hit.index])
		}
	case hitSoundBar:
		return m.setVolume(hit.index)
	case hitSoundVolumeDown:
		return m.setVolume(m.cfg.Volume - 5)
	case hitSoundVolumeUp:
		return m.setVolume(m.cfg.Volume + 5)
	case hitSoundPreview:
		return m.previewSound()
	case hitSoundMute:
		return m.toggleSound()
	case hitSoundClose:
		return m.closeSound()
	}
	return nil
}

func (m *Model) startTypedOrSelected() tea.Cmd {
	if strings.TrimSpace(m.input) != "" {
		d, err := duration.Parse(m.input)
		if err != nil {
			m.inputErr = copy.T(copy.ErrorDuration)
			return nil
		}
		m.input = ""
		m.inputErr = ""
		return m.send(protocol.ClientMessage{
			Type:    protocol.TypeStart,
			Seconds: int(d.Seconds()),
			Label:   duration.Short(d),
		})
	}
	if p, ok := m.selectedPreset(); ok {
		return m.startPreset(p)
	}
	m.inputErr = copy.T(copy.ErrorNoPreset)
	return nil
}

func (m *Model) startPreset(p config.Preset) tea.Cmd {
	if !config.ValidSeconds(p.Seconds) {
		m.showToast(copy.T(copy.ErrorDuration))
		return nil
	}
	return m.send(protocol.ClientMessage{
		Type:     protocol.TypeStart,
		Seconds:  p.Seconds,
		Label:    p.Label,
		PresetID: p.ID,
	})
}

func (m *Model) togglePause() tea.Cmd {
	switch m.status() {
	case engine.Running:
		return m.send(protocol.ClientMessage{Type: protocol.TypePause})
	case engine.Paused:
		return m.send(protocol.ClientMessage{Type: protocol.TypeResume})
	}
	return nil
}

func (m *Model) adjust(seconds int) tea.Cmd {
	switch m.status() {
	case engine.Running, engine.Paused:
		return m.send(protocol.ClientMessage{Type: protocol.TypeAdjust, DeltaSeconds: seconds})
	}
	return nil
}

func (m *Model) cycleTheme() tea.Cmd {
	next := theme.Next(m.cfg.Theme)
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.cfg.Theme = next
	m.pal = theme.Get(next)
	m.showToast(copy.T(copy.ToastTheme, next))
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetTheme, Name: next})
}

func (m *Model) openSound() {
	m.modal = modalSound
	m.soundCursor = soundIndex(m.cfg.SoundName)
}

func (m *Model) closeSound() tea.Cmd {
	m.modal = modalNone
	if !m.connected {
		return nil
	}
	return m.send(protocol.ClientMessage{Type: protocol.TypeStopSound})
}

func (m *Model) selectSound(name string) tea.Cmd {
	if !config.ValidSound(name) {
		return nil
	}
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.soundCursor = soundIndex(name)
	if m.cfg.SoundName == name {
		return m.previewSound()
	}
	m.cfg.SoundName = name
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetSoundName, SoundName: name})
}

func (m *Model) setVolume(volume int) tea.Cmd {
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	if volume < config.MinVolume {
		volume = config.MinVolume
	}
	if volume > config.MaxVolume {
		volume = config.MaxVolume
	}
	volume = ((volume + 2) / 5) * 5
	if volume == m.cfg.Volume {
		return nil
	}
	m.cfg.Volume = volume
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetVolume, Volume: &volume})
}

func (m *Model) previewSound() tea.Cmd {
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	return m.send(protocol.ClientMessage{Type: protocol.TypePreviewSound})
}

func (m *Model) toggleSound() tea.Cmd {
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	next := !m.cfg.Sound
	m.cfg.Sound = next
	if next {
		m.showToast(copy.T(copy.ToastSoundOn))
	} else {
		m.showToast(copy.T(copy.ToastSoundOff))
	}
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetSound, Enabled: &next})
}

func (m *Model) send(msg protocol.ClientMessage) tea.Cmd {
	if !m.connected || m.client == nil {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	c := m.client
	return func() tea.Msg { return sentMsg{err: c.Send(msg)} }
}

func (m *Model) openPresets() {
	m.modal = modalPresets
	m.presetCursor = 0
	m.presetScroll = 0
	m.presetFocus = presetFocusList
	if len(m.cfg.Presets) == 0 {
		m.presetCursor = 0
	} else if m.selected >= 0 && m.selected < len(m.cfg.Presets) {
		m.presetCursor = m.selected
	}
}

func (m *Model) openForm(mode formMode, p config.Preset) {
	m.modal = modalPresetForm
	m.form = formState{
		mode:     mode,
		id:       p.ID,
		label:    p.Label,
		duration: formDuration(p.Seconds),
		focus:    0,
	}
	if mode == formAdd {
		m.form.label = ""
		m.form.duration = ""
	}
}

func (m *Model) saveForm() tea.Cmd {
	label := strings.TrimSpace(m.form.label)
	if !config.ValidLabel(label) {
		m.form.err = copy.T(copy.ErrorLabel)
		return nil
	}
	d, err := duration.Parse(m.form.duration)
	if err != nil {
		m.form.err = copy.T(copy.ErrorDuration)
		return nil
	}
	if !m.connected {
		m.form.err = copy.T(copy.ErrorOffline)
		return nil
	}
	seconds := int(d.Seconds())
	if m.form.mode == formAdd {
		if len(m.cfg.Presets) >= config.MaxPresets {
			m.form.err = copy.T(copy.ErrorPresets)
			return nil
		}
		m.showToast(copy.T(copy.ToastAdded, label))
		m.modal = modalPresets
		m.form.err = ""
		return m.send(protocol.ClientMessage{Type: protocol.TypePresetAdd, Label: label, Seconds: seconds})
	}
	m.showToast(copy.T(copy.ToastUpdated, label))
	m.modal = modalPresets
	m.form.err = ""
	return m.send(protocol.ClientMessage{
		Type:    protocol.TypePresetUpdate,
		ID:      m.form.id,
		Label:   label,
		Seconds: seconds,
	})
}

func (m *Model) confirmDelete() tea.Cmd {
	if !m.connected {
		m.modal = modalPresets
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	id := m.deleteTarget.ID
	m.modal = modalPresets
	m.showToast(copy.T(copy.ToastDeleted))
	m.deleteTarget = config.Preset{}
	return m.send(protocol.ClientMessage{Type: protocol.TypePresetDelete, ID: id})
}

func (m *Model) closeModal() {
	switch m.modal {
	case modalHelp, modalConfirmStop:
		m.modal = modalNone
	case modalConfirmDelete, modalPresetForm:
		m.modal = modalPresets
	case modalPresets:
		m.modal = modalNone
	default:
		m.modal = modalNone
	}
	m.form.err = ""
}

func (m *Model) appendInput(s string) {
	if len(m.input)+len(s) > 16 {
		return
	}
	m.input += s
	m.inputErr = ""
}

func (m *Model) acceptsUnit() bool {
	if m.input == "" {
		return false
	}
	hasDigit := false
	for _, r := range m.input {
		if isDigitRune(r) {
			hasDigit = true
		}
	}
	if !hasDigit {
		return false
	}
	last := m.input[len(m.input)-1]
	return last != 'h' && last != 'm' && last != 's'
}

func (m *Model) selectedPreset() (config.Preset, bool) {
	if len(m.cfg.Presets) == 0 {
		return config.Preset{}, false
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.cfg.Presets) {
		m.selected = len(m.cfg.Presets) - 1
	}
	return m.cfg.Presets[m.selected], true
}

func (m *Model) cursorPreset() (config.Preset, bool) {
	if len(m.cfg.Presets) == 0 {
		return config.Preset{}, false
	}
	m.clampPresetCursor()
	return m.cfg.Presets[m.presetCursor], true
}

func (m *Model) clampPresetCursor() {
	if len(m.cfg.Presets) == 0 {
		m.presetCursor = 0
		m.presetScroll = 0
		return
	}
	if m.presetCursor < 0 {
		m.presetCursor = 0
	}
	if m.presetCursor >= len(m.cfg.Presets) {
		m.presetCursor = len(m.cfg.Presets) - 1
	}
}

func (m *Model) status() engine.Status {
	switch m.state.Status {
	case string(engine.Running):
		return engine.Running
	case string(engine.Paused):
		return engine.Paused
	case string(engine.Finished):
		return engine.Finished
	default:
		return engine.Idle
	}
}

func (m *Model) remaining() time.Duration {
	left := time.Duration(m.state.RemainingMS) * time.Millisecond
	if m.connected && m.state.Status == string(engine.Running) && m.hasState {
		left -= m.now().Sub(m.snapAt)
	}
	if left < 0 {
		left = 0
	}
	return left
}

func (m *Model) total() time.Duration {
	return time.Duration(m.state.DurationMS) * time.Millisecond
}

func (m *Model) progress() float64 {
	total := m.total()
	if total <= 0 {
		return 0
	}
	left := m.remaining()
	if left > total {
		left = total
	}
	p := 1 - float64(left)/float64(total)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func (m *Model) finishLabel() string {
	if label := strings.TrimSpace(m.state.Label); label != "" {
		return label
	}
	return duration.Short(m.total())
}

func (m *Model) toastActive() bool {
	return m.toast != "" && m.now().Before(m.toastUntil)
}

func (m *Model) viewSize() (int, int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

// Close releases the pipe connection. Called after the Bubble Tea program
// exits so the daemon keeps running.
func (m *Model) Close() {
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}
	m.connected = false
}

// HasActiveTimer reports whether the TUI is leaving a running or paused timer
// behind in the tray.
func (m *Model) HasActiveTimer() bool {
	switch m.status() {
	case engine.Running, engine.Paused:
		return true
	}
	return false
}

func formDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	if seconds%60 == 0 {
		return strconv.Itoa(seconds / 60)
	}
	return duration.Format(time.Duration(seconds) * time.Second)
}

func dropLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func isDigitText(s string) bool {
	if len([]rune(s)) != 1 {
		return false
	}
	return isDigitRune([]rune(s)[0])
}

func isDigitRune(r rune) bool { return r >= '0' && r <= '9' }

func soundIndex(name string) int {
	for i, s := range config.Sounds {
		if s == name {
			return i
		}
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
