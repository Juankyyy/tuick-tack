package daemon

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/update"
)

func TestUpdateCache(t *testing.T) {
	d := &Daemon{logger: log.New(io.Discard, "", 0)}
	if latest, url, ok := d.UpdateAvailable(); ok || latest != "" || url != "" {
		t.Fatalf("empty daemon reports an update: %q %q %v", latest, url, ok)
	}
	if p := d.Update(); p != nil {
		t.Fatalf("empty daemon payload = %+v", p)
	}
	if state, current := d.UpdateCheck(); state != "idle" || current != buildinfo.Version {
		t.Fatalf("initial check state = %q, %q", state, current)
	}

	info := update.Info{
		Version: "v0.2.0",
		URL:     "https://github.com/Juankyyy/tuick-tack/releases/tag/v0.2.0",
	}
	d.setUpdate(info)

	latest, url, ok := d.UpdateAvailable()
	if !ok || latest != info.Version || url != info.URL {
		t.Fatalf("UpdateAvailable = %q, %q, %v", latest, url, ok)
	}
	p := d.Update()
	if p == nil || p.Latest != info.Version || p.URL != info.URL {
		t.Fatalf("Update = %+v", p)
	}

	// A repeated result is cached, not re-announced; it must not panic without
	// a server or a tray.
	d.setUpdate(info)

	d.clearUpdate()
	if _, _, ok := d.UpdateAvailable(); ok {
		t.Fatal("clearUpdate left an available release")
	}
}

func TestManualUpdateCheck(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	defer func() { buildinfo.Version = oldVersion }()

	tag := "v1.0.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://example.test/releases/tag/%s"}`, tag, tag)
	}))
	defer srv.Close()

	d := &Daemon{
		logger:  log.New(io.Discard, "", 0),
		checker: &update.Checker{Client: srv.Client(), URL: srv.URL},
	}

	d.checkUpdates(nil, false)
	if state, _ := d.UpdateCheck(); state != "up_to_date" {
		t.Fatalf("up-to-date state = %q", state)
	}
	if _, _, ok := d.UpdateAvailable(); ok {
		t.Fatal("up-to-date check reports an available release")
	}

	tag = "v9.9.9"
	d.checkUpdates(nil, false)
	if state, _ := d.UpdateCheck(); state != "idle" {
		t.Fatalf("available state = %q", state)
	}
	if latest, _, ok := d.UpdateAvailable(); !ok || latest != tag {
		t.Fatalf("UpdateAvailable = %q, %v", latest, ok)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer bad.Close()
	d.checker.URL = bad.URL
	d.checkUpdates(nil, false)
	if state, _ := d.UpdateCheck(); state != "failed" {
		t.Fatalf("failed state = %q", state)
	}
	if latest, _, ok := d.UpdateAvailable(); !ok || latest != tag {
		t.Fatal("a failed check should keep the previous notice")
	}
}

func TestManualUpdateNotifications(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	defer func() { buildinfo.Version = oldVersion }()

	tag := "v0.1.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://example.test/releases/tag/%s"}`, tag, tag)
	}))
	defer srv.Close()

	var calls []struct{ title, message, launch string }
	d := &Daemon{
		logger:  log.New(io.Discard, "", 0),
		checker: &update.Checker{Client: srv.Client(), URL: srv.URL},
		notifier: func(title, message, launch string) error {
			calls = append(calls, struct{ title, message, launch string }{title, message, launch})
			return nil
		},
	}

	d.checkUpdates(nil, true)
	if len(calls) != 1 {
		t.Fatalf("up-to-date notifications = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.title != copy.T(copy.NotifyUpToDateTitle) ||
		!strings.Contains(call.message, "v1.0.0") || call.launch != "" {
		t.Fatalf("up-to-date notification = %+v", call)
	}

	calls = nil
	tag = "v9.9.9"
	d.checkUpdates(nil, true)
	if len(calls) != 1 {
		t.Fatalf("found notifications = %d, want 1", len(calls))
	}
	call = calls[0]
	if call.title != copy.T(copy.NotifyFoundTitle) ||
		!strings.Contains(call.message, "v9.9.9") ||
		call.launch != "https://example.test/releases/tag/v9.9.9" {
		t.Fatalf("found notification = %+v", call)
	}

	calls = nil
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer bad.Close()
	d.checker.URL = bad.URL
	d.checkUpdates(nil, true)
	if len(calls) != 1 || calls[0].title != copy.T(copy.NotifyFailedTitle) {
		t.Fatalf("failed notification = %+v", calls)
	}

	calls = nil
	d.checkUpdates(nil, false)
	if len(calls) != 0 {
		t.Fatalf("TUI checks must not raise a system notification: %+v", calls)
	}
}
