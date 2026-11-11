package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
)

func TestClientRoundTrip(t *testing.T) {
	enabled := true
	msgs := []ClientMessage{
		{Type: TypeHello, Protocol: Version},
		{Type: TypeStart, Seconds: 300, Label: "5 min", PresetID: "p5"},
		{Type: TypeAdjust, DeltaSeconds: 60},
		{Type: TypePresetAdd, Label: "Focus", Seconds: 1500},
		{Type: TypePresetUpdate, ID: "abc", Label: "Deep work", Seconds: 1500},
		{Type: TypePresetDelete, ID: "abc"},
		{Type: TypeSetTheme, Name: config.ThemeEmber},
		{Type: TypeSetSound, Enabled: &enabled},
		{Type: TypePause},
	}
	for _, want := range msgs {
		data, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal %+v: %v", want, err)
		}
		got, err := DecodeClient(data)
		if err != nil {
			t.Fatalf("DecodeClient(%s): %v", data, err)
		}
		if got.Type != want.Type || got.Seconds != want.Seconds ||
			got.Label != want.Label || got.ID != want.ID || got.Name != want.Name ||
			got.DeltaSeconds != want.DeltaSeconds {
			t.Fatalf("round trip %s -> %+v", data, got)
		}
		if want.Enabled != nil && (got.Enabled == nil || *got.Enabled != *want.Enabled) {
			t.Fatalf("enabled did not round trip: %+v", got)
		}
	}
}

func TestStateMessage(t *testing.T) {
	end := time.UnixMilli(1762839120000)
	st := engine.State{
		Status:    engine.Running,
		Duration:  5 * time.Minute,
		Remaining: 271500 * time.Millisecond,
		Label:     "5 min",
		PresetID:  "p5",
		FinishSeq: 3,
		EndAt:     end,
	}
	m := NewState(st)
	if m.Type != TypeState || m.Status != "running" || m.DurationMS != 300000 ||
		m.RemainingMS != 271500 || m.FinishSeq != 3 || m.EndAtUnixMS != 1762839120000 {
		t.Fatalf("unexpected state message: %+v", m)
	}
	data, _ := json.Marshal(m)
	back, err := DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer: %v", err)
	}
	sm, ok := back.(StateMessage)
	if !ok || sm != m {
		t.Fatalf("state did not round trip: %#v", back)
	}
}

func TestStateWithoutDeadline(t *testing.T) {
	m := NewState(engine.State{Status: engine.Paused, Duration: time.Minute, Remaining: time.Minute})
	if m.EndAtUnixMS != 0 {
		t.Fatalf("paused state should not carry an end time: %+v", m)
	}
}

func TestHelloAndConfigRoundTrip(t *testing.T) {
	c := config.Default()
	c.Theme = config.ThemeMono
	hello := NewHello("dev", c)
	data, _ := json.Marshal(hello)
	back, err := DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer: %v", err)
	}
	h, ok := back.(HelloMessage)
	if !ok || h.Protocol != Version || h.Config.Theme != config.ThemeMono ||
		len(h.Config.Presets) != 3 {
		t.Fatalf("unexpected hello: %#v", back)
	}

	cfg := NewConfig(c)
	data, _ = json.Marshal(cfg)
	back, err = DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer config: %v", err)
	}
	cm, ok := back.(ConfigMessage)
	if !ok || cm.Sound != c.Sound || len(cm.Presets) != 3 {
		t.Fatalf("unexpected config: %#v", back)
	}
}

func TestErrorRoundTrip(t *testing.T) {
	e := NewError(CodeInvalidDuration, "nope")
	data, _ := json.Marshal(e)
	back, err := DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer: %v", err)
	}
	em, ok := back.(ErrorMessage)
	if !ok || em.Code != CodeInvalidDuration || em.Message != "nope" {
		t.Fatalf("unexpected error: %#v", back)
	}
}

func TestDecodeServerUnknown(t *testing.T) {
	if _, err := DecodeServer([]byte(`{"type":"nope"}`)); err == nil {
		t.Fatal("expected an error for an unknown message type")
	}
	if _, err := DecodeServer([]byte(`{`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestValidate(t *testing.T) {
	enabled := true
	disabled := false
	tests := []struct {
		name string
		msg  ClientMessage
		code string
	}{
		{"hello", ClientMessage{Type: TypeHello, Protocol: Version}, ""},
		{"bad protocol", ClientMessage{Type: TypeHello, Protocol: Version + 1}, CodeUnsupportedProtocol},
		{"start", ClientMessage{Type: TypeStart, Seconds: 300, Label: "5 min", PresetID: "p5"}, ""},
		{"start bad seconds", ClientMessage{Type: TypeStart, Seconds: 0, Label: "5 min"}, CodeInvalidDuration},
		{"start bad label", ClientMessage{Type: TypeStart, Seconds: 300, Label: " "}, CodeInvalidLabel},
		{"start bad preset", ClientMessage{Type: TypeStart, Seconds: 300, Label: "x", PresetID: "bad id"}, CodeInvalidCommand},
		{"pause", ClientMessage{Type: TypePause}, ""},
		{"adjust", ClientMessage{Type: TypeAdjust, DeltaSeconds: -60}, ""},
		{"adjust zero", ClientMessage{Type: TypeAdjust}, CodeInvalidDuration},
		{"add", ClientMessage{Type: TypePresetAdd, Label: "Focus", Seconds: 1500}, ""},
		{"add bad label", ClientMessage{Type: TypePresetAdd, Label: "", Seconds: 1500}, CodeInvalidLabel},
		{"add bad duration", ClientMessage{Type: TypePresetAdd, Label: "Focus", Seconds: 86401}, CodeInvalidDuration},
		{"update", ClientMessage{Type: TypePresetUpdate, ID: "p5", Label: "Five", Seconds: 300}, ""},
		{"update missing id", ClientMessage{Type: TypePresetUpdate, Label: "Five", Seconds: 300}, CodeInvalidCommand},
		{"delete", ClientMessage{Type: TypePresetDelete, ID: "p5"}, ""},
		{"delete missing id", ClientMessage{Type: TypePresetDelete}, CodeInvalidCommand},
		{"restore", ClientMessage{Type: TypePresetsRestoreDefault}, ""},
		{"theme", ClientMessage{Type: TypeSetTheme, Name: config.ThemeEmber}, ""},
		{"bad theme", ClientMessage{Type: TypeSetTheme, Name: "neon"}, CodeInvalidTheme},
		{"sound on", ClientMessage{Type: TypeSetSound, Enabled: &enabled}, ""},
		{"sound off", ClientMessage{Type: TypeSetSound, Enabled: &disabled}, ""},
		{"sound missing", ClientMessage{Type: TypeSetSound}, CodeInvalidCommand},
		{"get state", ClientMessage{Type: TypeGetState}, ""},
		{"quit", ClientMessage{Type: TypeQuitDaemon}, ""},
		{"empty", ClientMessage{}, CodeInvalidMessage},
		{"unknown", ClientMessage{Type: "wat"}, CodeInvalidCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if tt.code == "" {
				if err != nil {
					t.Fatalf("Validate() = %+v, want nil", err)
				}
				return
			}
			if err == nil || err.Code != tt.code || err.Type != TypeError {
				t.Fatalf("Validate() = %+v, want code %q", err, tt.code)
			}
		})
	}
}
