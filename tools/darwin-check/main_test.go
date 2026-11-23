package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.txt")
	content := strings.Join([]string{
		"# header comment",
		"tuick-tack/internal/ipc      # named pipes",
		"",
		"tuick-tack/internal/tray # systray needs cgo",
		"tuick-tack/internal/ipc",
		"   tuick-tack/internal/wt   ",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readBaseline(path)
	if err != nil {
		t.Fatalf("readBaseline: %v", err)
	}
	want := []string{"tuick-tack/internal/ipc", "tuick-tack/internal/tray", "tuick-tack/internal/wt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readBaseline = %v, want %v", got, want)
	}
}

func TestReadBaselineMissing(t *testing.T) {
	if _, err := readBaseline(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("readBaseline accepted a missing file")
	}
}

func TestDiffSets(t *testing.T) {
	tests := []struct {
		name        string
		known, got  []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name: "equal",
			known: []string{
				"tuick-tack/internal/ipc",
			},
			got: []string{"tuick-tack/internal/ipc"},
		},
		{
			name:      "new failure",
			known:     []string{"tuick-tack/internal/ipc"},
			got:       []string{"tuick-tack/internal/ipc", "tuick-tack/internal/tui"},
			wantAdded: []string{"tuick-tack/internal/tui"},
		},
		{
			name:        "fixed package",
			known:       []string{"tuick-tack/internal/ipc", "tuick-tack/internal/tui"},
			got:         []string{"tuick-tack/internal/ipc"},
			wantRemoved: []string{"tuick-tack/internal/tui"},
		},
		{
			name:        "both",
			known:       []string{"tuick-tack/internal/ipc", "tuick-tack/internal/tui"},
			got:         []string{"tuick-tack/internal/browser", "tuick-tack/internal/tui"},
			wantAdded:   []string{"tuick-tack/internal/browser"},
			wantRemoved: []string{"tuick-tack/internal/ipc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := diffSets(tt.known, tt.got)
			if !reflect.DeepEqual(added, tt.wantAdded) {
				t.Errorf("added = %v, want %v", added, tt.wantAdded)
			}
			if !reflect.DeepEqual(removed, tt.wantRemoved) {
				t.Errorf("removed = %v, want %v", removed, tt.wantRemoved)
			}
		})
	}
}

func TestPackageDirs(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"root.go",
		".git/hooks/x.go",
		"_ignored/x.go",
		"testdata/x.go",
		"vendor/x/y.go",
		"assets/embed.go",
		"internal/a/a.go",
		"internal/a/a_test.go",
		"internal/b/b_test.go",
		"cmd/app/main.go",
	}
	for _, name := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := packageDirs(root)
	if err != nil {
		t.Fatalf("packageDirs: %v", err)
	}
	local := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
	want := []string{
		local(""),
		local("assets"),
		local("cmd/app"),
		local("internal/a"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packageDirs = %v, want %v", got, want)
	}
}

func TestBuildArgAndPackagePath(t *testing.T) {
	root := filepath.Join("repo")
	dir := filepath.Join("repo", "internal", "ipc")
	if got, want := buildArg(root, dir), "./internal/ipc"; got != want {
		t.Errorf("buildArg = %q, want %q", got, want)
	}
	if got, want := buildArg(root, root), "."; got != want {
		t.Errorf("buildArg(root) = %q, want %q", got, want)
	}
	if got, want := packagePath(root, dir), "internal/ipc"; got != want {
		t.Errorf("packagePath = %q, want %q", got, want)
	}
}

func TestFirstReason(t *testing.T) {
	out := "# tuick-tack/internal/browser\n\n\tpackage tuick-tack/internal/browser\n\timports golang.org/x/sys/windows: build constraints exclude all Go files\n"
	if got, want := firstReason([]byte(out)), "imports golang.org/x/sys/windows: build constraints exclude all Go files"; got != want {
		t.Fatalf("firstReason = %q, want %q", got, want)
	}
	if got := firstReason([]byte("# heading only\n")); got != "" {
		t.Fatalf("firstReason = %q, want empty", got)
	}
}

func TestCrossEnvOverridesTarget(t *testing.T) {
	t.Setenv("GOOS", "windows")
	t.Setenv("GOARCH", "amd64")
	t.Setenv("CGO_ENABLED", "1")

	env := crossEnv("arm64")
	counts := map[string]int{}
	for _, kv := range env {
		switch kv {
		case "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0":
			counts[kv]++
		case "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=1":
			t.Fatalf("crossEnv kept an inherited target variable: %s", kv)
		}
	}
	for _, want := range []string{"GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0"} {
		if counts[want] != 1 {
			t.Errorf("crossEnv set %s %d times, want 1", want, counts[want])
		}
	}
}
