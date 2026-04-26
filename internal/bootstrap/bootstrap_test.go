package bootstrap_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scriptPath returns the absolute path to scripts/bootstrap.sh, derived from this
// test file's location so the test runs correctly from any working directory.
func scriptPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(repoRoot, "scripts", "bootstrap.sh")
}

// runBootstrap invokes the bootstrap script with the given environment overrides
// and returns combined output and exit code. Environment overrides replace any
// existing values for those keys; other env vars are inherited from the parent.
func runBootstrap(t *testing.T, env map[string]string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", scriptPath(t))
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	output, err := cmd.CombinedOutput()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errAs(err, &ee) {
			exit = ee.ExitCode()
		} else {
			t.Fatalf("unexpected exec error: %v\noutput:\n%s", err, output)
		}
	}
	return string(output), exit
}

// errAs is a tiny errors.As wrapper kept local so the test file has no external
// dependencies beyond the standard library.
func errAs(err error, target any) bool {
	type errorAs interface{ As(any) bool }
	if a, ok := err.(errorAs); ok {
		return a.As(target)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if t, ok := target.(**exec.ExitError); ok {
			*t = ee
			return true
		}
	}
	return false
}

// fakeGoBin writes a stub `go` shim to a temp dir that prints the requested
// version string when called as `go version`. Returns the path to the shim.
func fakeGoBin(t *testing.T, versionLine string) string {
	t.Helper()
	dir := t.TempDir()
	shim := filepath.Join(dir, "go")
	contents := "#!/usr/bin/env bash\nif [ \"$1\" = \"version\" ]; then\n  echo \"" + versionLine + "\"\nfi\n"
	if err := os.WriteFile(shim, []byte(contents), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	return shim
}

// existingForkPath returns a usable typescript-go fork path for happy-path tests.
// Tests that need to vary this can pass their own value.
func existingForkPath(t *testing.T) string {
	t.Helper()
	candidate := filepath.Join(os.Getenv("HOME"), "src", "typescript-go")
	if _, err := os.Stat(candidate); err != nil {
		t.Skipf("typescript-go fork not present at %s (set up local dev to enable this test)", candidate)
	}
	return candidate
}

func TestBootstrap_ReportsGoToolchainAndForkWhenBothPresent(t *testing.T) {
	output, exit := runBootstrap(t, map[string]string{
		"TSGOLINT_FORK_PATH":       existingForkPath(t),
		"TSGOLINT_BOOTSTRAP_PHASE": "check",
	})
	if exit != 0 {
		t.Fatalf("expected exit 0, got %d. output:\n%s", exit, output)
	}
	if !strings.Contains(output, "Go toolchain:") {
		t.Errorf("expected Go toolchain line in output, got:\n%s", output)
	}
	if !strings.Contains(output, "typescript-go fork:") {
		t.Errorf("expected fork detection line in output, got:\n%s", output)
	}
}

func TestBootstrap_HaltsWhenForkPathIsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-not-a-fork")
	output, exit := runBootstrap(t, map[string]string{
		"TSGOLINT_FORK_PATH": missing,
	})
	if exit == 0 {
		t.Fatalf("expected non-zero exit, got 0. output:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(output), "typescript-go fork") {
		t.Errorf("expected error message naming the typescript-go fork prerequisite, got:\n%s", output)
	}
}

func TestBootstrap_HaltsWhenGoToolchainTooOld(t *testing.T) {
	shim := fakeGoBin(t, "go version go1.20.0 linux/amd64")
	output, exit := runBootstrap(t, map[string]string{
		"TSGOLINT_GO_BIN":          shim,
		"TSGOLINT_FORK_PATH":       existingForkPath(t),
		"TSGOLINT_BOOTSTRAP_PHASE": "check",
	})
	if exit == 0 {
		t.Fatalf("expected non-zero exit, got 0. output:\n%s", output)
	}
	out := strings.ToLower(output)
	if !strings.Contains(out, "go") || !strings.Contains(out, "1.20") {
		t.Errorf("expected error naming Go and the detected version, got:\n%s", output)
	}
}

func TestBootstrap_BuildPhaseProducesAnExecutableThatReportsItsVersion(t *testing.T) {
	binDir := t.TempDir()
	output, exit := runBootstrap(t, map[string]string{
		"TSGOLINT_FORK_PATH":       existingForkPath(t),
		"TSGOLINT_BIN_DIR":         binDir,
		"TSGOLINT_BOOTSTRAP_PHASE": "build",
	})
	if exit != 0 {
		t.Fatalf("expected exit 0, got %d. output:\n%s", exit, output)
	}
	if !strings.Contains(output, "Build complete") {
		t.Errorf("expected 'Build complete' in output, got:\n%s", output)
	}
	bin := filepath.Join(binDir, "tsgolint")
	if info, err := os.Stat(bin); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable at %s: %v", bin, err)
	}
	versionOut, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("built binary --version exited non-zero: %v\noutput:\n%s", err, versionOut)
	}
	if strings.TrimSpace(string(versionOut)) == "" {
		t.Errorf("expected non-empty version output from built binary")
	}
}

func TestBootstrap_HaltsBeforeBuildStepWhenPrerequisiteMissing(t *testing.T) {
	// The bootstrap should not produce any "build" output when prerequisites fail.
	missing := filepath.Join(t.TempDir(), "no-fork")
	output, exit := runBootstrap(t, map[string]string{
		"TSGOLINT_FORK_PATH": missing,
	})
	if exit == 0 {
		t.Fatalf("expected non-zero exit when prerequisites fail")
	}
	if strings.Contains(strings.ToLower(output), "building") || strings.Contains(strings.ToLower(output), "build complete") {
		t.Errorf("bootstrap proceeded to a build step despite missing prerequisite. output:\n%s", output)
	}
}
