package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// repoRoot returns the absolute path to the linter repository root, derived
// from this test file's location so the test runs from any working dir.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// buildBinary compiles tsgolint into a temp dir and returns the path.
func buildBinary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "tsgolint")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/tsgolint")
	cmd.Dir = repoRoot(t)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, combined)
	}
	return out
}

// fixtureProject creates a minimal TS project (tsconfig + one file) and
// returns the project root.
func fixtureProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.ts"), []byte("export {};\n"), 0o644); err != nil {
		t.Fatalf("write main.ts: %v", err)
	}
	return root
}

// runtimeDir returns an isolated XDG_RUNTIME_DIR for the test, so spawned
// daemons do not collide with developer machines.
func runtimeDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestCLI_SpawnsDaemonAndReportsOkOnCleanProject(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	cmd := exec.Command(bin, filepath.Join(project, "main.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("first invocation failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Errorf("expected 'ok' in stdout, got: %s", stdout.String())
	}

	// A daemon socket should now exist under the temp runtime dir.
	tsgolintDir := filepath.Join(rt, "tsgolint")
	entries, err := os.ReadDir(tsgolintDir)
	if err != nil {
		t.Fatalf("read tsgolint runtime dir: %v", err)
	}
	socketCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sock") {
			socketCount++
		}
	}
	if socketCount != 1 {
		t.Errorf("expected exactly one socket file, got %d", socketCount)
	}

	// Cleanup: tell any running daemon to exit by removing its idle window
	// indirectly is impossible; rely on the test's t.Cleanup-removed
	// runtime directory and the daemon's idle timeout to take it down
	// after the test, which is fine because the runtime dir is unique to
	// this test.
	_ = stderr // referenced above
	_ = time.Second
}

func TestCLI_SecondInvocationReusesTheRunningDaemon(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	first := exec.Command(bin, filepath.Join(project, "main.ts"))
	first.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	if out, err := first.CombinedOutput(); err != nil {
		t.Fatalf("first invocation failed: %v\n%s", err, out)
	}

	// Capture the daemon PID by reading the pid file, then run a second
	// invocation and confirm the same daemon answers.
	tsgolintDir := filepath.Join(rt, "tsgolint")
	entries, err := os.ReadDir(tsgolintDir)
	if err != nil {
		t.Fatalf("read tsgolint runtime dir: %v", err)
	}
	var pidFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pid") {
			pidFile = filepath.Join(tsgolintDir, e.Name())
			break
		}
	}
	if pidFile == "" {
		t.Fatal("no pid file found after first invocation")
	}
	pidBefore, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}

	second := exec.Command(bin, filepath.Join(project, "main.ts"))
	second.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	if out, err := second.CombinedOutput(); err != nil {
		t.Fatalf("second invocation failed: %v\n%s", err, out)
	}

	pidAfter, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read pid file after second invocation: %v", err)
	}
	if string(pidBefore) != string(pidAfter) {
		t.Errorf("expected the same daemon to serve both requests; pid changed from %s to %s",
			strings.TrimSpace(string(pidBefore)), strings.TrimSpace(string(pidAfter)))
	}
}

func TestCLI_NoTsconfigExitsTwoWithGuidance(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "loose.ts"), []byte("export {};"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	rt := runtimeDir(t)

	cmd := exec.Command(bin, filepath.Join(dir, "loose.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit, got 0. stdout: %s", stdout.String())
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("expected exit code 2, got %d", exitErr.ExitCode())
	}
	out := strings.ToLower(stderr.String())
	if !strings.Contains(out, "tsconfig") {
		t.Errorf("expected stderr to mention tsconfig, got: %s", stderr.String())
	}
}
