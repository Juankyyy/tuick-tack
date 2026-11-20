// Package protocol defines the versioned JSON messages exchanged over the
// local named pipe.
package protocol

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
)

// Version is the wire protocol version. Bump it when a message change breaks
// compatibility between the TUI and the daemon.
const Version = buildinfo.Protocol

// Message types.
const (
	TypeHello   = "hello"
	TypeState   = "state"
	TypeConfig  = "config"
	TypeError   = "error"
	TypeStart   = "start"
	TypePause   = "pause"
	TypeResume  = "resume"
	TypeStop    = "stop"
	TypeDismiss = "dismiss"
	TypeRestart = "restart"
	TypeAdjust  = "adjust"

	TypePresetAdd             = "preset_add"
	TypePresetUpdate          = "preset_update"
	TypePresetDelete          = "preset_delete"
	TypePresetsRestoreDefault = "presets_restore_defaults"

	TypeSetTheme   = "set_theme"
	TypeSetSound   = "set_sound"
	TypeGetState   = "get_state"
	TypeQuitDaemon = "quit_daemon"

	TypeSetSoundName = "set_sound_name"
	TypeSetVolume    = "set_volume"
	TypePreviewSound = "preview_sound"
	TypeStopSound    = "stop_sound"
	TypeCheckUpdate  = "check_update"

	// TypeSetWindowSize records the TUI's preferred terminal size in cells and,
	// optionally, the window position in screen pixels. It is additive: the
	// hello's window object announces support, and a TUI that does not see it
	// never sends the command, so an older daemon is safe.
	TypeSetWindowSize = "set_window_size"

	// TypeInstallUpdate downloads the release installer and runs it after the
	// daemon and the TUI exit. Force is required when a countdown is active.
	TypeInstallUpdate = "install_update"
	// TypeDownloadPortable saves the portable executable in the user's
	// Downloads folder without restarting the app.
	TypeDownloadPortable = "download_portable"

	// TypeUpdate is an additive server message: clients that do not know it
	// ignore it (see DecodeServer), so it does not require a protocol bump.
	TypeUpdate = "update"
	// TypeUpdateResult is the reply to check_update and is only sent to the
	// client that asked.
	TypeUpdateResult = "update_result"
	// TypeUpdateProgress reports a setup or portable download. It is additive.
	TypeUpdateProgress = "update_progress"
	// TypeDaemonClosing is an additive server message: it tells connected TUIs
	// that the daemon is shutting down on purpose, so they exit instead of
	// reconnecting and launching a new daemon.
	TypeDaemonClosing = "daemon_closing"

	// TypeGetStats asks for the aggregated session history. It is additive: an
	// older daemon answers invalid_command and the TUI closes the tab again.
	TypeGetStats = "get_stats"
	// TypeStats is the reply to get_stats and is sent only to the client that
	// asked, like update_result.
	TypeStats = "stats"
)

// Reasons carried by TypeDaemonClosing.
const (
	CloseReasonQuit   = "quit"
	CloseReasonUpdate = "update"
)

// Error codes.
const (
	CodeInvalidMessage      = "invalid_message"
	CodeInvalidCommand      = "invalid_command"
	CodeInvalidDuration     = "invalid_duration"
	CodeInvalidLabel        = "invalid_label"
	CodeInvalidTheme        = "invalid_theme"
	CodeInvalidSound        = "invalid_sound"
	CodeInvalidVolume       = "invalid_volume"
	CodeInvalidWindowSize   = "invalid_window_size"
	CodePresetLimit         = "preset_limit"
	CodePresetNotFound      = "preset_not_found"
	CodeUnsupportedProtocol = "unsupported_protocol"
	CodeConfigWrite         = "config_write_failed"
	CodeInternal            = "internal"
)

// ClientMessage is the union of every TUI-to-daemon command. Fields not used
// by a command are omitted on the wire.
type ClientMessage struct {
	Type         string                 `json:"type"`
	Protocol     int                    `json:"protocol,omitempty"`
	Seconds      int                    `json:"seconds,omitempty"`
	Label        string                 `json:"label,omitempty"`
	PresetID     string                 `json:"preset_id,omitempty"`
	DeltaSeconds int                    `json:"delta_seconds,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	SoundName    string                 `json:"sound_name,omitempty"`
	Volume       *int                   `json:"volume,omitempty"`
	Enabled      *bool                  `json:"enabled,omitempty"`
	Width        int                    `json:"width,omitempty"`
	Height       int                    `json:"height,omitempty"`
	Position     *config.WindowPosition `json:"position,omitempty"`
	Force        bool                   `json:"force,omitempty"`
}

// ConfigPayload is the shared preset, theme, sound and volume shape.
type ConfigPayload struct {
	Presets   []config.Preset `json:"presets"`
	Theme     string          `json:"theme"`
	Sound     bool            `json:"sound"`
	SoundName string          `json:"sound_name"`
	Volume    int             `json:"volume"`
}

// ServerMessage is implemented by every daemon-to-client message.
type ServerMessage interface {
	isServerMessage()
}

// HelloMessage is the server's handshake reply.
type HelloMessage struct {
	Type       string             `json:"type"`
	Protocol   int                `json:"protocol"`
	AppVersion string             `json:"app_version"`
	Config     ConfigPayload      `json:"config"`
	Update     *UpdatePayload     `json:"update,omitempty"`
	Window     *WindowSizePayload `json:"window,omitempty"`
}

func (HelloMessage) isServerMessage() {}

// WindowSizePayload is the TUI's preferred terminal size in character cells and
// the optional position of the window's outer top-left in screen pixels. A
// hello from this build always carries the object (zeros when no size is
// stored), so its presence also tells the TUI that set_window_size is
// supported. Older daemons omit the position and older TUIs ignore it.
type WindowSizePayload struct {
	Width    int                    `json:"width"`
	Height   int                    `json:"height"`
	Position *config.WindowPosition `json:"position,omitempty"`
}

// UpdatePayload is the latest release the daemon knows about. It is carried by
// the handshake and by the update broadcast. Setup and Portable say which
// download actions the release supports. Notes is the release body (Markdown)
// trimmed to a display-sized cap; it is additive, so an older client ignores it
// and an older daemon simply omits it.
type UpdatePayload struct {
	Latest   string `json:"latest"`
	URL      string `json:"url"`
	Setup    bool   `json:"setup,omitempty"`
	Portable bool   `json:"portable,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// UpdateMessage announces a newer release. The TUI and the tray turn it into a
// link; nothing is downloaded.
type UpdateMessage struct {
	Type string `json:"type"`
	UpdatePayload
}

func (UpdateMessage) isServerMessage() {}

// UpdateResultMessage answers a manual check. Error is non-empty when GitHub
// could not be reached; the TUI then keeps the previous notice untouched.
type UpdateResultMessage struct {
	Type     string `json:"type"`
	Current  string `json:"current"`
	Latest   string `json:"latest,omitempty"`
	URL      string `json:"url,omitempty"`
	Setup    bool   `json:"setup,omitempty"`
	Portable bool   `json:"portable,omitempty"`
	Notes    string `json:"notes,omitempty"`
	Error    string `json:"error,omitempty"`
}

// UpdateProgressMessage reports one update download. Kind is "setup" or
// "portable". Phase is "downloading", "verifying", "installing", "done" or
// "failed". Path is set when a portable executable was saved, and Error is a
// short code ("network", "checksum", "timer_active", ...).
type UpdateProgressMessage struct {
	Type     string `json:"type"`
	Kind     string `json:"kind"`
	Phase    string `json:"phase"`
	Received int64  `json:"received,omitempty"`
	Total    int64  `json:"total,omitempty"`
	Path     string `json:"path,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (UpdateProgressMessage) isServerMessage() {}

func (UpdateResultMessage) isServerMessage() {}

// DaemonClosingMessage tells a connected TUI that the daemon is shutting down
// because the user quit it. The TUI exits without reconnecting; reconnecting
// would start a new daemon.
type DaemonClosingMessage struct {
	Type   string `json:"type"`
	Reason string `json:"reason,omitempty"`
}

func (DaemonClosingMessage) isServerMessage() {}

// StatsMessage answers get_stats with the daemon's aggregated history. The
// payload is precomputed so the TUI only has to draw it.
type StatsMessage struct {
	Type  string       `json:"type"`
	Stats StatsPayload `json:"stats"`
}

func (StatsMessage) isServerMessage() {}

// StatsPayload is the whole Statistics tab in one message (~8 KiB).
type StatsPayload struct {
	// Daily covers the last 189 local days, oldest first.
	Daily []StatsDay `json:"daily"`
	// Hourly has 24 buckets by local start hour.
	Hourly []StatsBucket `json:"hourly"`
	// Weekday has 7 buckets, Monday first.
	Weekday []StatsBucket `json:"weekday"`
	// Labels are the most used labels by total focus time.
	Labels []StatsLabel `json:"labels"`
	// Records are the all-time highlights.
	Records StatsRecords `json:"records"`
	// Recent holds the newest sessions, newest first.
	Recent []StatsEvent `json:"recent"`
	// Totals are the precomputed range summaries.
	Totals StatsTotals `json:"totals"`
	// Retained is how many records the store kept.
	Retained int `json:"retained"`
}

// StatsDay is one local calendar day of focus time.
type StatsDay struct {
	Date string `json:"date"`
	MS   int64  `json:"ms"`
	N    int    `json:"n"`
}

// StatsBucket aggregates time and session count for an hour or weekday.
type StatsBucket struct {
	MS int64 `json:"ms"`
	N  int   `json:"n"`
}

// StatsLabel groups the sessions that shared a label.
type StatsLabel struct {
	Label     string `json:"label"`
	MS        int64  `json:"ms"`
	N         int    `json:"n"`
	Completed int    `json:"completed"`
	Stopped   int    `json:"stopped"`
}

// StatsEvent is one session in the recent list.
type StatsEvent struct {
	StartedMS int64  `json:"started_ms"`
	PlannedMS int64  `json:"planned_ms"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Label     string `json:"label"`
	Outcome   string `json:"outcome"`
}

// StatsTotals holds one summary per selectable range.
type StatsTotals struct {
	All StatsRange `json:"all"`
	D7  StatsRange `json:"d7"`
	D30 StatsRange `json:"d30"`
	D90 StatsRange `json:"d90"`
}

// StatsRange is a session summary for a time window. MS sums the real elapsed
// time, so sessions that were stopped early still count what they focused.
type StatsRange struct {
	Sessions  int   `json:"sessions"`
	Completed int   `json:"completed"`
	Stopped   int   `json:"stopped"`
	Replaced  int   `json:"replaced"`
	MS        int64 `json:"ms"`
}

// StatsRecords are the all-time highlights shown in the overview.
type StatsRecords struct {
	StreakDays     int    `json:"streak_days"`
	BestStreakDays int    `json:"best_streak_days"`
	LongestMS      int64  `json:"longest_ms"`
	LongestLabel   string `json:"longest_label,omitempty"`
	// BusiestWeekday is Monday=0..Sunday=6, or -1 with no sessions.
	BusiestWeekday int `json:"busiest_weekday"`
	// BusiestHour is 0..23, or -1 with no sessions.
	BusiestHour int `json:"busiest_hour"`
}

// IgnoredMessage is returned by DecodeServer for a message type this build does
// not know. Additive server messages must not break older clients.
type IgnoredMessage struct{}

func (IgnoredMessage) isServerMessage() {}

// StateMessage is the flattened countdown snapshot.
type StateMessage struct {
	Type        string `json:"type"`
	Status      string `json:"status"`
	DurationMS  int64  `json:"duration_ms"`
	RemainingMS int64  `json:"remaining_ms"`
	Label       string `json:"label,omitempty"`
	PresetID    string `json:"preset_id,omitempty"`
	FinishSeq   int64  `json:"finish_seq"`
	EndAtUnixMS int64  `json:"end_at_unix_ms,omitempty"`
}

func (StateMessage) isServerMessage() {}

// ConfigMessage is broadcast when presets, theme or sound change.
type ConfigMessage struct {
	Type string `json:"type"`
	ConfigPayload
}

func (ConfigMessage) isServerMessage() {}

// ErrorMessage reports a rejected command without changing state.
type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (ErrorMessage) isServerMessage() {}

// NewHello builds the handshake reply.
func NewHello(appVersion string, c config.Config, upd *UpdatePayload) HelloMessage {
	m := HelloMessage{
		Type:       TypeHello,
		Protocol:   Version,
		AppVersion: appVersion,
		Config:     NewConfigPayload(c),
		Window: &WindowSizePayload{
			Width:  c.WindowWidth,
			Height: c.WindowHeight,
		},
	}
	if c.WindowPosition != nil {
		pos := *c.WindowPosition
		m.Window.Position = &pos
	}
	if upd != nil {
		payload := *upd
		m.Update = &payload
	}
	return m
}

// NewUpdate builds the update broadcast.
func NewUpdate(upd UpdatePayload) UpdateMessage {
	return UpdateMessage{Type: TypeUpdate, UpdatePayload: upd}
}

// NewUpdateResult builds the reply to a manual check.
func NewUpdateResult(res UpdateResultMessage) UpdateResultMessage {
	res.Type = TypeUpdateResult
	return res
}

// NewDaemonClosing builds the shutdown notice sent to connected TUIs. The
// optional reason tells an updating TUI that the app will reopen.
func NewDaemonClosing(reason ...string) DaemonClosingMessage {
	r := CloseReasonQuit
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	return DaemonClosingMessage{Type: TypeDaemonClosing, Reason: r}
}

// NewUpdateProgress builds a download progress notice.
func NewUpdateProgress(m UpdateProgressMessage) UpdateProgressMessage {
	m.Type = TypeUpdateProgress
	return m
}

// NewStats builds the reply to get_stats.
func NewStats(p StatsPayload) StatsMessage {
	return StatsMessage{Type: TypeStats, Stats: p}
}

// NewState converts an engine snapshot to the wire shape.
func NewState(st engine.State) StateMessage {
	m := StateMessage{
		Type:        TypeState,
		Status:      string(st.Status),
		DurationMS:  st.Duration.Milliseconds(),
		RemainingMS: st.Remaining.Milliseconds(),
		Label:       st.Label,
		PresetID:    st.PresetID,
		FinishSeq:   st.FinishSeq,
	}
	if !st.EndAt.IsZero() {
		m.EndAtUnixMS = st.EndAt.UnixMilli()
	}
	return m
}

// NewConfigPayload copies the configuration into the wire shape.
func NewConfigPayload(c config.Config) ConfigPayload {
	return ConfigPayload{
		Presets:   append([]config.Preset(nil), c.Presets...),
		Theme:     c.Theme,
		Sound:     c.Sound,
		SoundName: c.SoundName,
		Volume:    c.Volume,
	}
}

// NewConfig builds a config broadcast.
func NewConfig(c config.Config) ConfigMessage {
	return ConfigMessage{Type: TypeConfig, ConfigPayload: NewConfigPayload(c)}
}

// NewError builds an error reply.
func NewError(code, message string) ErrorMessage {
	return ErrorMessage{Type: TypeError, Code: code, Message: message}
}

// DecodeClient parses one client message.
func DecodeClient(data []byte) (ClientMessage, error) {
	var m ClientMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return ClientMessage{}, fmt.Errorf("decode message: %w", err)
	}
	return m, nil
}

// DecodeServer parses one server message based on its "type" field. Unknown
// types decode to IgnoredMessage instead of failing, so an additive message
// does not disconnect an older client. Malformed JSON is still an error.
func DecodeServer(data []byte) (ServerMessage, error) {
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}
	switch env.Type {
	case TypeHello:
		var m HelloMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeState:
		var m StateMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeConfig:
		var m ConfigMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeError:
		var m ErrorMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeUpdate:
		var m UpdateMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeUpdateResult:
		var m UpdateResultMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeUpdateProgress:
		var m UpdateProgressMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeDaemonClosing:
		var m DaemonClosingMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	case TypeStats:
		var m StatsMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return m, nil
	default:
		return IgnoredMessage{}, nil
	}
}

// Validate checks a client message before it reaches the engine.
func (m ClientMessage) Validate() *ErrorMessage {
	invalid := func(code, msg string) *ErrorMessage {
		e := NewError(code, msg)
		return &e
	}
	switch m.Type {
	case TypeHello:
		if m.Protocol != Version {
			return invalid(CodeUnsupportedProtocol,
				fmt.Sprintf("protocol %d is not supported (need %d)", m.Protocol, Version))
		}
		return nil
	case TypeStart:
		if !config.ValidSeconds(m.Seconds) {
			return invalid(CodeInvalidDuration, "duration must be between 1 second and 24 hours")
		}
		if !config.ValidLabel(m.Label) {
			return invalid(CodeInvalidLabel, "label must be 1..32 printable characters")
		}
		if m.PresetID != "" && !config.ValidPresetID(m.PresetID) {
			return invalid(CodeInvalidCommand, "invalid preset id")
		}
		return nil
	case TypePause, TypeResume, TypeStop, TypeDismiss, TypeRestart,
		TypePresetsRestoreDefault, TypeGetState, TypeGetStats, TypeQuitDaemon,
		TypePreviewSound, TypeStopSound, TypeCheckUpdate,
		TypeInstallUpdate, TypeDownloadPortable:
		return nil
	case TypeAdjust:
		if m.DeltaSeconds == 0 || math.Abs(float64(m.DeltaSeconds)) > config.MaxSeconds {
			return invalid(CodeInvalidDuration, "adjustment must be between -86400 and 86400 seconds")
		}
		return nil
	case TypePresetAdd:
		if !config.ValidLabel(m.Label) {
			return invalid(CodeInvalidLabel, "label must be 1..32 printable characters")
		}
		if !config.ValidSeconds(m.Seconds) {
			return invalid(CodeInvalidDuration, "duration must be between 1 second and 24 hours")
		}
		return nil
	case TypePresetUpdate:
		if !config.ValidPresetID(m.ID) {
			return invalid(CodeInvalidCommand, "invalid preset id")
		}
		if !config.ValidLabel(m.Label) {
			return invalid(CodeInvalidLabel, "label must be 1..32 printable characters")
		}
		if !config.ValidSeconds(m.Seconds) {
			return invalid(CodeInvalidDuration, "duration must be between 1 second and 24 hours")
		}
		return nil
	case TypePresetDelete:
		if !config.ValidPresetID(m.ID) {
			return invalid(CodeInvalidCommand, "invalid preset id")
		}
		return nil
	case TypeSetTheme:
		if !config.ValidTheme(m.Name) {
			return invalid(CodeInvalidTheme, fmt.Sprintf("unknown theme %q", m.Name))
		}
		return nil
	case TypeSetSound:
		if m.Enabled == nil {
			return invalid(CodeInvalidCommand, "set_sound requires enabled")
		}
		return nil
	case TypeSetSoundName:
		if !config.ValidSound(m.SoundName) {
			return invalid(CodeInvalidSound, fmt.Sprintf("unknown sound %q", m.SoundName))
		}
		return nil
	case TypeSetVolume:
		if m.Volume == nil {
			return invalid(CodeInvalidCommand, "set_volume requires volume")
		}
		if !config.ValidVolume(*m.Volume) {
			return invalid(CodeInvalidVolume,
				fmt.Sprintf("volume must be between %d and %d", config.MinVolume, config.MaxVolume))
		}
		return nil
	case TypeSetWindowSize:
		if m.Width < config.MinWindowCols || m.Width > config.MaxWindowCols ||
			m.Height < config.MinWindowRows || m.Height > config.MaxWindowRows {
			return invalid(CodeInvalidWindowSize,
				fmt.Sprintf("window size must be %d..%d by %d..%d",
					config.MinWindowCols, config.MaxWindowCols,
					config.MinWindowRows, config.MaxWindowRows))
		}
		if m.Position != nil && !config.ValidWindowPosition(*m.Position) {
			return invalid(CodeInvalidWindowSize,
				fmt.Sprintf("window position must be %d..%d by %d..%d",
					config.MinWindowPos, config.MaxWindowPos,
					config.MinWindowPos, config.MaxWindowPos))
		}
		return nil
	case "":
		return invalid(CodeInvalidMessage, "missing message type")
	default:
		return invalid(CodeInvalidCommand, fmt.Sprintf("unknown command %q", m.Type))
	}
}

// ValidLabel is a convenience alias used by tests and the daemon.
func ValidLabel(label string) bool { return config.ValidLabel(strings.TrimSpace(label)) }
