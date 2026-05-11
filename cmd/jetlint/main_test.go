package main_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltBinary_RunsAndReportsVersion verifies the built binary boots and
// reports a version. End-to-end coverage of the build path; behavioral coverage
// of the CLI lives in internal/cli where Run is testable in-process.
func TestBuiltBinary_RunsAndReportsVersion(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version on built binary failed: %v\noutput:\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Errorf("expected non-empty version output from built binary")
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "jetlint")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/jetlint")
	cmd.Dir = repoRoot(t)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build failed: %v\nstderr:\n%s", err, stderr.String())
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("locate go.mod: %v", err)
	}
	return filepath.Dir(strings.TrimSpace(string(out)))
}
