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
	CodePresetLimit         = "preset_limit"
	CodePresetNotFound      = "preset_not_found"
	CodeUnsupportedProtocol = "unsupported_protocol"
	CodeConfigWrite         = "config_write_failed"
	CodeInternal            = "internal"
)

// ClientMessage is the union of every TUI-to-daemon command. Fields not used
// by a command are omitted on the wire.
type ClientMessage struct {
	Type         string `json:"type"`
	Protocol     int    `json:"protocol,omitempty"`
	Seconds      int    `json:"seconds,omitempty"`
	Label        string `json:"label,omitempty"`
	PresetID     string `json:"preset_id,omitempty"`
	DeltaSeconds int    `json:"delta_seconds,omitempty"`
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	SoundName    string `json:"sound_name,omitempty"`
	Volume       *int   `json:"volume,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
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
	Type       string        `json:"type"`
	Protocol   int           `json:"protocol"`
	AppVersion string        `json:"app_version"`
	Config     ConfigPayload `json:"config"`
}

func (HelloMessage) isServerMessage() {}

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
func NewHello(appVersion string, c config.Config) HelloMessage {
	return HelloMessage{
		Type:       TypeHello,
		Protocol:   Version,
		AppVersion: appVersion,
		Config:     NewConfigPayload(c),
	}
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

// DecodeServer parses one server message based on its "type" field.
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
	default:
		return nil, fmt.Errorf("unknown server message type %q", env.Type)
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
		TypePresetsRestoreDefault, TypeGetState, TypeQuitDaemon,
		TypePreviewSound, TypeStopSound:
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
	case "":
		return invalid(CodeInvalidMessage, "missing message type")
	default:
		return invalid(CodeInvalidCommand, fmt.Sprintf("unknown command %q", m.Type))
	}
}

// ValidLabel is a convenience alias used by tests and the daemon.
func ValidLabel(label string) bool { return config.ValidLabel(strings.TrimSpace(label)) }
