package duration

import (
	"errors"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"25", 25 * time.Minute},
		{"25m", 25 * time.Minute},
		{"90s", 90 * time.Second},
		{"1h30m", 90 * time.Minute},
		{"1h", time.Hour},
		{"1h30m20s", 90*time.Minute + 20*time.Second},
		{"12:30", 12*time.Minute + 30*time.Second},
		{"1:02:03", time.Hour + 2*time.Minute + 3*time.Second},
		{"0:59", 59 * time.Second},
		{"24h", 24 * time.Hour},
		{" 10m ", 10 * time.Minute},
		{"1H30M", 90 * time.Minute},
		{"1s", time.Second},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []string{
		"", " ", "0", "-5", "1.5", "1,5", "abc", "5x", "1h1h", "1h30",
		"12:60", "1:60:03", ":30", "12:", "1:2:3:4", "0s", "25:70",
		"1441", "25h", "86401s", "1h30m30x", "10 20",
	}
	for _, in := range tests {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", in)
		} else if !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalid or ErrOutOfRange", in, err)
		}
	}
}

func TestParseOutOfRange(t *testing.T) {
	for _, in := range []string{"1441m", "86401s", "25h"} {
		if _, err := Parse(in); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Parse(%q) error = %v, want ErrOutOfRange", in, err)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00"},
		{time.Second, "00:01"},
		{59 * time.Second, "00:59"},
		{60 * time.Second, "01:00"},
		{12*time.Minute + 30*time.Second, "12:30"},
		{59*time.Minute + 59*time.Second, "59:59"},
		{time.Hour, "1:00:00"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1:02:03"},
		{24 * time.Hour, "24:00:00"},
		{1500 * time.Millisecond, "00:02"},
		{-time.Second, "00:00"},
	}
	for _, tt := range tests {
		if got := Format(tt.in); got != tt.want {
			t.Errorf("Format(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestShort(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{5 * time.Minute, "5 min"},
		{90 * time.Second, "1:30"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h 30m"},
		{90*time.Minute + 5*time.Second, "1h 30m 5s"},
		{0, "0s"},
	}
	for _, tt := range tests {
		if got := Short(tt.in); got != tt.want {
			t.Errorf("Short(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
