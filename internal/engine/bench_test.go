package engine_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/nofloatingpromises"
)

// BenchmarkLint_500FileProject lints a generated 500-file TypeScript
// project and reports the per-iteration time. Use `go test -bench=. ./internal/engine`
// to run. The plan's warm-path budget is 200ms p95 / 400ms p99; this
// benchmark reports the median ns/op which CI gates can compare against
// historical runs.
func BenchmarkLint_500FileProject(b *testing.B) {
	dir := setupPerfFixture(b, 500)
	tsconfig := filepath.Join(dir, "tsconfig.json")

	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		b.Fatalf("LoadProgram: %v", err)
	}
	defer prog.Close()

	eng := engine.New(
		[]engine.Rule{nofloatingpromises.New()},
		map[string]wrapperlint.Severity{"no-floating-promises": wrapperlint.SeverityError},
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eng.Lint(prog)
	}
}

// TestLint_WarmPathBudgetOn500FileProject runs the warm-path lint twenty
// times against a 500-file fixture and fails the build if either the
// 95th-percentile or 99th-percentile invocation exceeds the plan's
// budget. The fixture is loaded once before the timed loop so this
// measures the warm path rather than cold-start.
func TestLint_WarmPathBudgetOn500FileProject(t *testing.T) {
	if testing.Short() {
		t.Skip("perf budget test is slow; run with `go test -timeout 60s ./internal/engine`")
	}
	dir := setupPerfFixtureT(t, 500)
	tsconfig := filepath.Join(dir, "tsconfig.json")
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)

	eng := engine.New(
		[]engine.Rule{nofloatingpromises.New()},
		map[string]wrapperlint.Severity{"no-floating-promises": wrapperlint.SeverityError},
	)

	const samples = 20
	durations := make([]time.Duration, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		_ = eng.Lint(prog)
		durations[i] = time.Since(start)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	// 95th percentile is the 19th element of the sorted 20-sample slice
	// (0-indexed index 18 yields a value with at most 1/20 = 5% larger).
	p95 := durations[18]
	p99 := durations[19]
	const p95Budget = 200 * time.Millisecond
	const p99Budget = 400 * time.Millisecond
	t.Logf("warm-path p95=%s p99=%s over %d samples on a 500-file project",
		p95, p99, samples)
	if p95 > p95Budget {
		t.Errorf("warm-path p95 %s exceeds budget %s", p95, p95Budget)
	}
	if p99 > p99Budget {
		t.Errorf("warm-path p99 %s exceeds budget %s", p99, p99Budget)
	}
}

// TestLint_ColdStartBudgetOn500FileProject measures the cold-start
// path (Program load + first lint pass) and asserts it completes
// within the plan's cold-start budget of 5 seconds.
func TestLint_ColdStartBudgetOn500FileProject(t *testing.T) {
	if testing.Short() {
		t.Skip("perf budget test is slow")
	}
	dir := setupPerfFixtureT(t, 500)
	tsconfig := filepath.Join(dir, "tsconfig.json")

	start := time.Now()
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nofloatingpromises.New()},
		map[string]wrapperlint.Severity{"no-floating-promises": wrapperlint.SeverityError},
	)
	_ = eng.Lint(prog)
	elapsed := time.Since(start)

	const budget = 5 * time.Second
	t.Logf("cold-start (LoadProgram + first lint) on 500-file project: %s", elapsed)
	if elapsed > budget {
		t.Errorf("cold-start %s exceeds budget %s", elapsed, budget)
	}
}

func setupPerfFixtureT(t *testing.T, n int) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsgperf")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	writePerfFixture(t, dir, n)
	return dir
}

func writePerfFixture(t testing.TB, dir string, n int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`),
		0o644); err != nil {
		t.Fatalf("tsconfig: %v", err)
	}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("mod%04d.ts", i)
		var content string
		if i == 0 {
			content = "export async function fn(): Promise<number> { return 1; }\n"
		} else {
			prev := fmt.Sprintf("./mod%04d", i-1)
			content = fmt.Sprintf(
				"import { fn as prev } from %q;\nexport async function fn(): Promise<number> { return (await prev()) + 1; }\n",
				prev,
			)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// setupPerfFixture writes a synthetic project with N source files into a
// temp directory and returns the directory. Each file is small but
// includes a cross-module call chain so the type checker has real work
// to do.
func setupPerfFixture(b *testing.B, n int) string {
	b.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsgperf")
	if err != nil {
		b.Fatalf("mkdir: %v", err)
	}
	b.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`),
		0o644); err != nil {
		b.Fatalf("tsconfig: %v", err)
	}

	// Each file exports a small async function and imports the previous
	// file. This forces the checker to resolve cross-file types
	// representative of a real project.
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("mod%04d.ts", i)
		var content string
		if i == 0 {
			content = "export async function fn(): Promise<number> { return 1; }\n"
		} else {
			prev := fmt.Sprintf("./mod%04d", i-1)
			content = fmt.Sprintf(
				"import { fn as prev } from %q;\nexport async function fn(): Promise<number> { return (await prev()) + 1; }\n",
				prev,
			)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			b.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}
