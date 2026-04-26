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
	// Default human formatter mirrors biome's behavior on a clean
	// project: emit a "Checked N files in Tms. No fixes applied."
	// summary so users get positive confirmation the run completed.
	if !strings.Contains(stdout.String(), "Checked") {
		t.Errorf("expected human summary line on clean project, got: %s", stdout.String())
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

func TestCLI_DetectsFloatingPromiseAcrossFilesAndExitsOne(t *testing.T) {
	bin := buildBinary(t)
	rt := runtimeDir(t)

	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	for path, content := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`,
		"api.ts":        "export async function saveUser(name: string): Promise<void> { return; }\n",
		"main.ts":       "import { saveUser } from \"./api\";\nsaveUser(\"alice\");\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	cmd := exec.Command(bin, "--format", "json", filepath.Join(dir, "main.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	exitErr, _ := err.(*exec.ExitError)
	if exitErr == nil || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1 for floating promise, got %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	var doc struct {
		SchemaVersion int `json:"schemaVersion"`
		Diagnostics   []map[string]any
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("decode output: %v\nstdout: %s", err, stdout.String())
	}
	if len(doc.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(doc.Diagnostics), doc.Diagnostics)
	}
	got := doc.Diagnostics[0]
	if got["ruleId"] != "no-floating-promises" {
		t.Errorf("expected ruleId 'no-floating-promises', got %v", got["ruleId"])
	}
	if !strings.HasSuffix(got["file"].(string), "main.ts") {
		t.Errorf("expected file to end with main.ts, got %v", got["file"])
	}
}

func TestCLI_ModifiedFileIsReParsedOnNextInvocation(t *testing.T) {
	bin := buildBinary(t)
	rt := runtimeDir(t)

	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	tsconfig := filepath.Join(dir, "tsconfig.json")
	main := filepath.Join(dir, "main.ts")
	if err := os.WriteFile(tsconfig, []byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`), 0o644); err != nil {
		t.Fatalf("tsconfig: %v", err)
	}

	// First version: clean (no violations).
	if err := os.WriteFile(main, []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatalf("main v1: %v", err)
	}
	first := exec.Command(bin, "--format", "json", main)
	first.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	out1, err := first.CombinedOutput()
	if err != nil {
		t.Fatalf("first run failed: %v\n%s", err, out1)
	}

	// Second version: introduces a floating promise. Same file path,
	// different content — possibly within the same modification-time
	// tick on a fast filesystem.
	if err := os.WriteFile(main,
		[]byte("async function fn(): Promise<void> { return; }\nfn();\n"), 0o644); err != nil {
		t.Fatalf("main v2: %v", err)
	}
	second := exec.Command(bin, "--format", "json", main)
	second.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	out2, err := second.CombinedOutput()
	exitErr, _ := err.(*exec.ExitError)
	if exitErr == nil || exitErr.ExitCode() != 1 {
		t.Fatalf("second run should have exited 1 after edit; got %v\n%s", err, out2)
	}
	if !strings.Contains(string(out2), "no-floating-promises") {
		t.Errorf("expected no-floating-promises in re-parsed output, got: %s", out2)
	}
}

func TestCLI_DegradedModeWarningWhenProgramHasTypeErrors(t *testing.T) {
	bin := buildBinary(t)
	rt := runtimeDir(t)

	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	// A program with a real type error: assigning a string literal to
	// a number-typed variable.
	for path, content := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`,
		"main.ts":       "export const x: number = \"not a number\";\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	cmd := exec.Command(bin, "--format", "json", filepath.Join(dir, "main.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // we don't assert exit here; the warning may or may not push exit to 1
	var doc struct {
		Diagnostics []map[string]any
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("decode output: %v\nstdout: %s", err, stdout.String())
	}
	foundWarning := false
	for _, d := range doc.Diagnostics {
		if d["ruleId"] == "tsgolint/program-has-type-errors" {
			foundWarning = true
			if d["severity"] != "warning" {
				t.Errorf("expected severity 'warning', got %v", d["severity"])
			}
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected degraded-mode warning, got: %v", doc.Diagnostics)
	}
}

func TestCLI_MaxDiagnosticsFlagTruncatesHumanOutput(t *testing.T) {
	bin := buildBinary(t)
	rt := runtimeDir(t)

	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	// A file with three floating-promise call sites generates three
	// diagnostics; --max-diagnostics=1 must render only one block and
	// surface a "Diagnostics not shown: 2." line.
	for path, content := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`,
		"main.ts": "" +
			"async function a(): Promise<void> { return; }\n" +
			"async function b(): Promise<void> { return; }\n" +
			"async function c(): Promise<void> { return; }\n" +
			"a();\nb();\nc();\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	cmd := exec.Command(bin, "--max-diagnostics", "1", filepath.Join(dir, "main.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // exit 1 expected; we only assert on output here

	out := stdout.String()
	if !strings.Contains(out, "Diagnostics not shown: 2") {
		t.Errorf("expected 'Diagnostics not shown: 2' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Found 3 errors.") {
		t.Errorf("expected 'Found 3 errors.' in summary (total, not truncated), got:\n%s", out)
	}
}

func TestCLI_BrokenTsconfigEmitsProgramBuildFailedError(t *testing.T) {
	bin := buildBinary(t)
	rt := runtimeDir(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{ this is not valid json `), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"),
		[]byte("export {};\n"), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	cmd := exec.Command(bin, "--format", "json", filepath.Join(dir, "main.ts"))
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, _ := err.(*exec.ExitError)
	if exitErr == nil || exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit 2 for broken tsconfig, got %v\nstderr: %s", err, stderr.String())
	}

	var got map[string]any
	if decErr := json.Unmarshal([]byte(strings.TrimSpace(stderr.String())), &got); decErr != nil {
		t.Fatalf("decode stderr: %v\nstderr: %s", decErr, stderr.String())
	}
	if got["code"] != "program_build_failed" {
		t.Errorf("expected code 'program_build_failed', got %v", got["code"])
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got: %s", stdout.String())
	}
}

func TestCLI_AcceptsMultiplePositionalTargets(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	// Add a second file to the fixture project so the multi-target test
	// has more than one path to pass.
	other := filepath.Join(project, "other.ts")
	if err := os.WriteFile(other, []byte("export {};\n"), 0o644); err != nil {
		t.Fatalf("write other.ts: %v", err)
	}

	cmd := exec.Command(bin, filepath.Join(project, "main.ts"), other)
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("multi-target invocation failed: %v\n%s", err, out)
	}
}

func TestCLI_AcceptsFileListFromStdin(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	cmd := exec.Command(bin, "--files-from", "-")
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	cmd.Stdin = strings.NewReader(filepath.Join(project, "main.ts") + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stdin file-list invocation failed: %v\n%s", err, out)
	}
}

func TestCLI_AcceptsFileListFromFile(t *testing.T) {
	bin := buildBinary(t)
	project := fixtureProject(t)
	rt := runtimeDir(t)

	listFile := filepath.Join(t.TempDir(), "files.txt")
	if err := os.WriteFile(listFile,
		[]byte(filepath.Join(project, "main.ts")+"\n"), 0o644); err != nil {
		t.Fatalf("write list: %v", err)
	}

	cmd := exec.Command(bin, "--files-from", listFile)
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+rt)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--files-from invocation failed: %v\n%s", err, out)
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
