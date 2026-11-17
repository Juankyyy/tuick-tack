package browser

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		bad  bool
	}{
		{"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0",
			"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0", false},
		{"http://example.com", "", true},
		{"file:///C:/Windows/System32/calc.exe", "", true},
		{"javascript:alert(1)", "", true},
		{"https:///no-host", "", true},
		{"", "", true},
		{"not a url", "", true},
	}
	for _, tt := range tests {
		got, err := validate(tt.raw)
		if tt.bad {
			if !errors.Is(err, ErrInvalidURL) {
				t.Errorf("validate(%q) error = %v, want ErrInvalidURL", tt.raw, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("validate(%q) = %q, %v; want %q", tt.raw, got, err, tt.want)
		}
	}
}
