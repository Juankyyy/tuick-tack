// Command darwin-check cross-builds every package of the module for macOS
// without cgo and compares the packages that fail with a documented baseline.
//
// Windows is still the reference platform, so part of the platform layer only
// builds on Windows. This check keeps that set of failures exact: a package
// that stops building for darwin is a new breakage, and a package that starts
// building must leave the baseline so the document cannot rot. It is the
// phase-0 guardrail of the macOS port; the CI macOS jobs run it on arm64 and
// amd64, and the same command works from a Windows checkout.
//
// Usage, from anywhere inside the module:
//
//	go run ./tools/darwin-check [-arch arm64] [-baseline path]
//
// The check covers package builds only. Test files are not compiled for
// darwin yet.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// failure is one package that does not build for darwin, with the first line
// of the compiler output as a hint for new entries.
type failure struct {
	pkg    string
	reason string
}

func main() {
	arch := flag.String("arch", "arm64", "GOARCH used for the cross-build")
	baseline := flag.String("baseline", "", "baseline file (default tools/darwin-check/baseline.txt)")
	flag.Parse()

	root, err := moduleRoot()
	if err != nil {
		exitf(2, "darwin-check: %v", err)
	}
	baselinePath := *baseline
	if baselinePath == "" {
		baselinePath = filepath.Join(root, "tools", "darwin-check", "baseline.txt")
	}
	known, err := readBaseline(baselinePath)
	if err != nil {
		exitf(2, "darwin-check: %v", err)
	}
	failures, err := buildFailures(root, *arch)
	if err != nil {
		exitf(2, "darwin-check: %v", err)
	}

	got := make([]string, len(failures))
	for i, f := range failures {
		got[i] = f.pkg
	}
	added, removed := diffSets(known, got)

	fmt.Printf("darwin-check: GOOS=darwin GOARCH=%s CGO_ENABLED=0\n", *arch)
	fmt.Printf("packages that do not build: %d (baseline: %d)\n", len(got), len(known))
	if len(added) == 0 && len(removed) == 0 {
		fmt.Printf("ok: matches %s\n", displayPath(root, baselinePath))
		return
	}

	if len(added) > 0 {
		fmt.Println("new failures:")
		for _, pkg := range added {
			fmt.Printf("  %s\n", pkg)
			if reason := reasonFor(failures, pkg); reason != "" {
				fmt.Printf("      %s\n", reason)
			}
		}
		fmt.Println("fix them or add them to the baseline if they are expected.")
	}
	if len(removed) > 0 {
		fmt.Println("packages that now build (remove them from the baseline):")
		for _, pkg := range removed {
			fmt.Printf("  %s\n", pkg)
		}
	}
	os.Exit(1)
}

func exitf(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(code)
}

// moduleRoot returns the directory that holds go.mod, so the check also works
// from a subdirectory.
func moduleRoot() (string, error) {
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMOD: %w", err)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == os.DevNull {
		return "", errors.New("not inside a Go module")
	}
	return filepath.Dir(gomod), nil
}

// buildFailures builds each package of the module for darwin and returns the
// ones that fail, sorted by package path.
func buildFailures(root, arch string) ([]failure, error) {
	dirs, err := packageDirs(root)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "darwin-check-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	out := filepath.Join(tmp, "out")
	failures := make([]failure, 0)
	for _, dir := range dirs {
		cmd := exec.Command("go", "build", "-o", out, buildArg(root, dir))
		cmd.Dir = root
		cmd.Env = crossEnv(arch)
		msg, err := cmd.CombinedOutput()
		if err == nil {
			continue
		}
		failures = append(failures, failure{pkg: packagePath(root, dir), reason: firstReason(msg)})
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].pkg < failures[j].pkg })
	return failures, nil
}

// packageDirs lists the directories of the module that Go would build, using
// the same rules as ./... (ignore hidden and underscore directories, testdata
// and vendor) plus the rule that a directory needs a non-test .go file. The
// walk does not use go list because a package with no files for the target
// GOOS disappears from ./... exactly when the check needs to see it.
func packageDirs(root string) ([]string, error) {
	dirs := make(map[string]bool)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		dirs[filepath.Dir(path)] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(dirs))
	for dir := range dirs {
		paths = append(paths, dir)
	}
	sort.Strings(paths)
	return paths, nil
}

// buildArg turns an absolute package directory into a ./path argument.
func buildArg(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return "."
	}
	return "./" + filepath.ToSlash(rel)
}

// packagePath returns the module-relative path of a package directory, which
// is what the baseline records.
func packagePath(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

// crossEnv returns the current environment with the darwin cross-build
// variables set, replacing any inherited value.
func crossEnv(goarch string) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		switch {
		case strings.HasPrefix(kv, "GOOS="),
			strings.HasPrefix(kv, "GOARCH="),
			strings.HasPrefix(kv, "CGO_ENABLED="):
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GOOS=darwin", "GOARCH="+goarch, "CGO_ENABLED=0")
}

// firstReason picks the first meaningful line of a compiler error, skipping
// package headings and empty lines.
func firstReason(out []byte) string {
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "package ") {
			continue
		}
		return line
	}
	return ""
}

func reasonFor(failures []failure, pkg string) string {
	for _, f := range failures {
		if f.pkg == pkg {
			return f.reason
		}
	}
	return ""
}

// readBaseline reads one package path per line. A # starts a comment, so the
// file can document why each package is still Windows-only.
func readBaseline(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]bool)
	var pkgs []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		pkgs = append(pkgs, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

// diffSets returns the entries only in got and only in want, sorted.
func diffSets(want, got []string) (added, removed []string) {
	inWant := make(map[string]bool, len(want))
	for _, pkg := range want {
		inWant[pkg] = true
	}
	inGot := make(map[string]bool, len(got))
	for _, pkg := range got {
		inGot[pkg] = true
		if !inWant[pkg] {
			added = append(added, pkg)
		}
	}
	for _, pkg := range want {
		if !inGot[pkg] {
			removed = append(removed, pkg)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func displayPath(root, path string) string {
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return path
		}
		path = abs
	}
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}
