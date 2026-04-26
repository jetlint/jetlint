package cli_test

import (
	"bytes"
	"encoding/json"
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

// runtimeDir returns an isolated XDG_RUNTIME_DIR for the test. It uses a
// short /tmp-prefixed path rather than t.TempDir because Unix domain
// socket paths have a ~108-byte limit on Linux, and t.TempDir embeds the
// (potentially long) test name which blows the limit for tests with
// descriptive names.
func runtimeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("create runtime dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
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
	// Default human formatter emits nothing on a clean project; the
	// success signal is exit code 0 plus an empty stdout.
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on clean project, got: %s", stdout.String())
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

func TestCLI_NoTsconfigInJSONModeEmitsStructuredError(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "loose.ts"), []byte("export {};"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	rt := runtimeDir(t)

	cmd := exec.Command(bin, "--format", "json", filepath.Join(dir, "loose.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit, got 0")
	}

	stderrStr := strings.TrimSpace(stderr.String())
	// One line, parseable JSON, code = tsconfig_missing.
	if strings.Count(stderrStr, "\n") != 0 {
		t.Errorf("expected single-line JSON on stderr, got: %q", stderrStr)
	}
	var got map[string]any
	if decErr := json.Unmarshal([]byte(stderrStr), &got); decErr != nil {
		t.Fatalf("decode stderr JSON: %v\nstderr: %s", decErr, stderrStr)
	}
	if got["code"] != "tsconfig_missing" {
		t.Errorf("expected code 'tsconfig_missing', got %v", got["code"])
	}
	if got["message"] == nil || got["message"] == "" {
		t.Errorf("expected non-empty message, got: %v", got["message"])
	}
}

func TestCLI_UnknownFormatExitsTwoWithSupportedFormatsListed(t *testing.T) {
	bin := buildBinary(t)
	rt := runtimeDir(t)

	cmd := exec.Command(bin, "--format", "yaml", "/tmp/doesnt-matter.ts")
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit, got 0")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("expected exit code 2, got %d", exitErr.ExitCode())
	}
	out := stderr.String()
	if !strings.Contains(out, "human") || !strings.Contains(out, "json") {
		t.Errorf("expected supported formats listed in stderr, got: %s", out)
	}
}

func TestCLI_DaemonWritesPerProjectLogFileToStateDirectory(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)
	state := t.TempDir()

	cmd := exec.Command(bin, filepath.Join(project, "main.ts"))
	cmd.Env = append(os.Environ(),
		"XDG_RUNTIME_DIR="+rt,
		"XDG_STATE_HOME="+state,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("invocation failed: %v\n%s", err, out)
	}

	tsgolintStateDir := filepath.Join(state, "tsgolint")
	entries, err := os.ReadDir(tsgolintStateDir)
	if err != nil {
		t.Fatalf("read state dir: %v", err)
	}
	var logFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log") {
			logFile = filepath.Join(tsgolintStateDir, e.Name())
			break
		}
	}
	if logFile == "" {
		t.Fatalf("no log file found in %s", tsgolintStateDir)
	}
	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(contents), "tsgolint daemon: started") {
		t.Errorf("expected start log line in log file, got: %s", string(contents))
	}
}

func TestCLI_StaleSocketIsDetectedAndReplaced(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	// Pre-populate the runtime dir with a stale "socket" file (a regular
	// file at the path; nothing listening). The CLI must remove it and
	// spawn a real daemon.
	tsgolintDir := filepath.Join(rt, "tsgolint")
	if err := os.MkdirAll(tsgolintDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Determine the would-be socket path the CLI will compute by running
	// once; capture it via the directory listing afterward.
	cmd := exec.Command(bin, filepath.Join(project, "main.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("first invocation failed: %v\n%s", err, out)
	}
	entries, err := os.ReadDir(tsgolintDir)
	if err != nil {
		t.Fatalf("read tsgolint dir: %v", err)
	}
	var socketPath string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sock") {
			socketPath = filepath.Join(tsgolintDir, e.Name())
			break
		}
	}
	if socketPath == "" {
		t.Fatal("could not find socket path after first invocation")
	}

	// Replace the socket with a stale regular file to simulate a crashed
	// daemon's leftover state.
	_ = os.Remove(socketPath)
	if err := os.WriteFile(socketPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}

	cmd2 := exec.Command(bin, filepath.Join(project, "main.ts"))
	cmd2.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	out, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("second invocation should recover from stale socket, got error: %v\n%s", err, out)
	}
}

func TestCLI_ConcurrentInvocationsBothSucceedAgainstTheSameDaemon(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	// Run an initial invocation to establish a warm daemon. This avoids
	// stressing the spawn-election path (which is exercised separately by
	// the unit-level flock test) and isolates this test to the
	// "concurrent requests against an existing daemon" property.
	warmup := exec.Command(bin, filepath.Join(project, "main.ts"))
	warmup.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	if out, err := warmup.CombinedOutput(); err != nil {
		t.Fatalf("warmup invocation failed: %v\n%s", err, out)
	}

	const N = 4
	type result struct {
		err    error
		output string
	}
	results := make(chan result, N)
	for i := 0; i < N; i++ {
		go func() {
			cmd := exec.Command(bin, filepath.Join(project, "main.ts"))
			cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
			out, err := cmd.CombinedOutput()
			results <- result{err: err, output: string(out)}
		}()
	}

	deadline := time.After(15 * time.Second)
	for i := 0; i < N; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Errorf("concurrent invocation %d failed: %v\n%s", i, r.err, r.output)
			}
		case <-deadline:
			t.Fatalf("concurrent invocations did not all complete within deadline")
		}
	}
}

func TestCLI_JSONModeOnCleanProjectEmitsValidJSON(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	cmd := exec.Command(bin, "--format", "json", filepath.Join(project, "main.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("invocation failed: %v\nstderr: %s", err, stderr.String())
	}
	var got map[string]any
	if decErr := json.Unmarshal(stdout.Bytes(), &got); decErr != nil {
		t.Fatalf("decode stdout JSON: %v\nstdout: %s", decErr, stdout.String())
	}
	if _, ok := got["schemaVersion"]; !ok {
		t.Errorf("expected schemaVersion in output, got: %v", got)
	}
	if diags, ok := got["diagnostics"].([]any); !ok || len(diags) != 0 {
		t.Errorf("expected empty diagnostics array, got: %v", got["diagnostics"])
	}
}
