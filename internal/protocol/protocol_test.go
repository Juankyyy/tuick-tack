package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
)

func TestClientRoundTrip(t *testing.T) {
	enabled := true
	volume := 35
	msgs := []ClientMessage{
		{Type: TypeHello, Protocol: Version},
		{Type: TypeStart, Seconds: 300, Label: "5 min", PresetID: "p5"},
		{Type: TypeAdjust, DeltaSeconds: 60},
		{Type: TypePresetAdd, Label: "Focus", Seconds: 1500},
		{Type: TypePresetUpdate, ID: "abc", Label: "Deep work", Seconds: 1500},
		{Type: TypePresetDelete, ID: "abc"},
		{Type: TypeSetTheme, Name: config.ThemeEmber},
		{Type: TypeSetSound, Enabled: &enabled},
		{Type: TypeSetSoundName, SoundName: config.SoundMarimba},
		{Type: TypeSetVolume, Volume: &volume},
		{Type: TypePreviewSound},
		{Type: TypeStopSound},
		{Type: TypeCheckUpdate},
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
			got.SoundName != want.SoundName ||
			got.DeltaSeconds != want.DeltaSeconds {
			t.Fatalf("round trip %s -> %+v", data, got)
		}
		if want.Enabled != nil && (got.Enabled == nil || *got.Enabled != *want.Enabled) {
			t.Fatalf("enabled did not round trip: %+v", got)
		}
		if want.Volume != nil && (got.Volume == nil || *got.Volume != *want.Volume) {
			t.Fatalf("volume did not round trip: %+v", got)
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
	upd := &UpdatePayload{Latest: "v0.2.0", URL: "https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0"}
	hello := NewHello("dev", c, upd)
	data, _ := json.Marshal(hello)
	back, err := DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer: %v", err)
	}
	h, ok := back.(HelloMessage)
	if !ok || h.Protocol != Version || h.Config.Theme != config.ThemeMono ||
		len(h.Config.Presets) != 3 || h.Config.SoundName != c.SoundName ||
		h.Config.Volume != c.Volume || h.Update == nil || h.Update.Latest != "v0.2.0" ||
		h.Update.URL != upd.URL {
		t.Fatalf("unexpected hello: %#v", back)
	}

	cfg := NewConfig(c)
	data, _ = json.Marshal(cfg)
	back, err = DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer config: %v", err)
	}
	cm, ok := back.(ConfigMessage)
	if !ok || cm.Sound != c.Sound || len(cm.Presets) != 3 ||
		cm.SoundName != c.SoundName || cm.Volume != c.Volume {
		t.Fatalf("unexpected config: %#v", back)
	}
}

func TestUpdateRoundTrip(t *testing.T) {
	upd := UpdatePayload{Latest: "v0.2.0", URL: "https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0"}
	data, _ := json.Marshal(NewUpdate(upd))
	back, err := DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer: %v", err)
	}
	um, ok := back.(UpdateMessage)
	if !ok || um.Type != TypeUpdate || um.Latest != upd.Latest || um.URL != upd.URL {
		t.Fatalf("unexpected update: %#v", back)
	}

	// The handshake may omit the update entirely.
	hello := NewHello("dev", config.Default(), nil)
	if hello.Update != nil {
		t.Fatalf("nil update should stay nil: %+v", hello.Update)
	}
	if data, _ := json.Marshal(hello); strings.Contains(string(data), `"update"`) {
		t.Fatalf("nil update should be omitted: %s", data)
	}
}

func TestUpdateResultRoundTrip(t *testing.T) {
	res := NewUpdateResult(UpdateResultMessage{
		Current: "v0.1.2",
		Latest:  "v0.2.0",
		URL:     "https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0",
	})
	data, _ := json.Marshal(res)
	back, err := DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer: %v", err)
	}
	rm, ok := back.(UpdateResultMessage)
	if !ok || rm.Type != TypeUpdateResult || rm.Current != res.Current ||
		rm.Latest != res.Latest || rm.URL != res.URL || rm.Error != "" {
		t.Fatalf("unexpected result: %#v", back)
	}

	failed := NewUpdateResult(UpdateResultMessage{Current: "v0.1.2", Error: "failed"})
	data, _ = json.Marshal(failed)
	back, err = DecodeServer(data)
	if err != nil {
		t.Fatalf("DecodeServer failure: %v", err)
	}
	rm, ok = back.(UpdateResultMessage)
	if !ok || rm.Error != "failed" || rm.Latest != "" {
		t.Fatalf("unexpected failure result: %#v", back)
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
	msg, err := DecodeServer([]byte(`{"type":"nope"}`))
	if err != nil {
		t.Fatalf("unknown types must decode to IgnoredMessage: %v", err)
	}
	if _, ok := msg.(IgnoredMessage); !ok {
		t.Fatalf("unknown type decoded to %#v", msg)
	}
	if _, err := DecodeServer([]byte(`{`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestValidate(t *testing.T) {
	enabled := true
	disabled := false
	volume := 35
	volumeZero := 0
	volumeLow := -1
	volumeHigh := 101
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
		{"sound name", ClientMessage{Type: TypeSetSoundName, SoundName: config.SoundDeep}, ""},
		{"bad sound name", ClientMessage{Type: TypeSetSoundName, SoundName: "airhorn"}, CodeInvalidSound},
		{"volume", ClientMessage{Type: TypeSetVolume, Volume: &volume}, ""},
		{"volume zero", ClientMessage{Type: TypeSetVolume, Volume: &volumeZero}, ""},
		{"volume low", ClientMessage{Type: TypeSetVolume, Volume: &volumeLow}, CodeInvalidVolume},
		{"volume high", ClientMessage{Type: TypeSetVolume, Volume: &volumeHigh}, CodeInvalidVolume},
		{"volume missing", ClientMessage{Type: TypeSetVolume}, CodeInvalidCommand},
		{"preview sound", ClientMessage{Type: TypePreviewSound}, ""},
		{"stop sound", ClientMessage{Type: TypeStopSound}, ""},
		{"check update", ClientMessage{Type: TypeCheckUpdate}, ""},
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
