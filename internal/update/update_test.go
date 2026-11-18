package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v0.1.2", "v0.1.2", 0},
		{"0.1.2", "v0.1.2", 0},
		{"v0.1.2", "v0.1.10", -1},
		{"v0.2.0", "v0.1.9", 1},
		{"v1.0.0-rc1", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0-rc1", 1},
		{"v1.0.0", "v1.0.0+build.7", 0},
		{"v1.0", "v1.0.0", 0},
		{"dev", "v0.1.2", -1},
		{"v0.1.2", "dev", 1},
		{"dev", "dev", 0},
		{"not a version", "v1.0.0", -1},
		{"v-1.0.0", "v0.0.0", -1},
	}
	for _, tt := range tests {
		if got := Compare(tt.a, tt.b); got != tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestDelay(t *testing.T) {
	if got := Delay(false); got != CheckInterval {
		t.Fatalf("Delay(false) = %v, want %v", got, CheckInterval)
	}
	if got := Delay(true); got != RetryInterval {
		t.Fatalf("Delay(true) = %v, want %v", got, RetryInterval)
	}
	if CheckInterval <= RetryInterval {
		t.Fatal("the regular interval should be longer than the retry interval")
	}
}

func TestCheckerLatest(t *testing.T) {
	var gotUA, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v0.2.0",
			"html_url": "https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0",
			"body": "## What's new in v0.2.0\r\n\r\n- Faster\r\n",
			"draft": false
		}`))
	}))
	defer srv.Close()

	c := Checker{Client: srv.Client(), URL: srv.URL, UserAgent: "tuick-tack/v0.1.2"}
	info, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if info.Version != "v0.2.0" || info.URL != "https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0" {
		t.Fatalf("info = %+v", info)
	}
	if !strings.Contains(info.Notes, "What's new") || !strings.Contains(info.Notes, "- Faster") {
		t.Fatalf("notes = %q", info.Notes)
	}
	if strings.Contains(info.Notes, "\r") {
		t.Fatalf("notes kept CRLF: %q", info.Notes)
	}
	if !strings.HasPrefix(gotUA, "tuick-tack/") {
		t.Fatalf("User-Agent = %q", gotUA)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
}

func TestCheckerLatestErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		ctx     func() (context.Context, context.CancelFunc)
	}{
		{
			name: "not found",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "nope", http.StatusNotFound)
			},
		},
		{
			name: "rate limited",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, "rate limited", http.StatusForbidden)
			},
		},
		{
			name: "bad json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{`))
			},
		},
		{
			name: "missing tag",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"html_url":"https://example.test"}`))
			},
		},
		{
			name: "missing url",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
			},
		},
		{
			name: "insecure url",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"http://example.test"}`))
			},
		},
		{
			name: "canceled",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			},
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			ctx := context.Background()
			if tt.ctx != nil {
				var cancel context.CancelFunc
				ctx, cancel = tt.ctx()
				defer cancel()
			}
			c := Checker{Client: srv.Client(), URL: srv.URL}
			if _, err := c.Latest(ctx); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestCheckerTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := Checker{
		Client: &http.Client{Timeout: 50 * time.Millisecond},
		URL:    srv.URL,
	}
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("expected a timeout")
	}
}

func TestCheckerMissingURL(t *testing.T) {
	if _, err := (Checker{}).Latest(context.Background()); err == nil {
		t.Fatal("expected an error for an empty URL")
	}
}

func TestCleanNotes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trim", "  hello  ", "hello"},
		{"crlf", "a\r\nb\rc", "a\nb\nc"},
		{"empty", " \r\n ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanNotes(tt.in); got != tt.want {
				t.Fatalf("cleanNotes(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCleanNotesCapsAtRuneBoundary(t *testing.T) {
	body := "a" + strings.Repeat("á", maxNotesBytes)
	got := cleanNotes(body)
	if len(got) > maxNotesBytes {
		t.Fatalf("notes length = %d, want <= %d", len(got), maxNotesBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatal("the capped notes are not valid UTF-8")
	}
	if len(got) < maxNotesBytes-2 {
		t.Fatalf("the cap cut too much: %d bytes", len(got))
	}
}
