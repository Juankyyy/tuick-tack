//go:build darwin

package updater

import (
	"path/filepath"
	"testing"
)

func TestDarwinUnsupported(t *testing.T) {
	if code := Apply([]string{"--apply-update"}); code != 2 {
		t.Fatalf("Apply = %d, want 2", code)
	}
	if err := Spawn("/tmp/setup", "/tmp/app", "/tmp/pipe"); err == nil {
		t.Fatal("Spawn should report that macOS has no helper")
	}
	dir, err := UpdateDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "updates" {
		t.Fatalf("UpdateDir = %q, want the cache updates directory", dir)
	}
}
