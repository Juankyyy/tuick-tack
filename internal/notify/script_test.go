package notify

import "testing"

func TestAppleScriptString(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Update available", `"Update available"`},
		{`say "hi"`, `"say \"hi\""`},
		{`C:\path`, `"C:\\path"`},
		{"line\nbreak", `"line break"`},
		{"two\r\nlines", `"two lines"`},
	}
	for _, tt := range tests {
		if got := appleScriptString(tt.in); got != tt.want {
			t.Errorf("appleScriptString(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
