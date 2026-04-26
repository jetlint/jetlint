package main_test

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// buildBinary compiles the tsgolint binary into a temp directory and returns
// its path. Tests use this helper instead of a pre-built binary so they can
// run from a clean state on any machine.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "tsgolint")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/tsgolint")
	cmd.Dir = repoRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\noutput:\n%s", err, output)
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

func TestVersion_PrintsSingleLineSemanticVersionAndExitsZero(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version exited non-zero: %v\noutput:\n%s", err, output)
	}
	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single line of output, got %d lines:\n%s", len(lines), output)
	}
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	if !semver.MatchString(strings.TrimSpace(lines[0])) {
		t.Errorf("output %q does not match semantic version shape", lines[0])
	}
}

func TestHelp_PrintsUsageAndExitsZero(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--help exited non-zero: %v\noutput:\n%s", err, output)
	}
	out := string(output)
	for _, want := range []string{"Usage", "tsgolint", "--version", "--help"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage output missing %q. got:\n%s", want, out)
		}
	}
}
