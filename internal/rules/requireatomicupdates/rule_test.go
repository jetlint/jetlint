package requireatomicupdates_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/requireatomicupdates"
)

func runRule(t *testing.T, code string) int {
	t.Helper()
	dir := t.TempDir()
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(`{"compilerOptions":{"strict":false,"target":"es2022","module":"esnext","moduleResolution":"bundler","allowJs":true,"noImplicitAny":false}}`), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(code), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{requireatomicupdates.New()},
		map[string]wrapperlint.Severity{"require-atomic-updates": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestRequireAtomicUpdates_FlagsReadAcrossAwait(t *testing.T) {
	if n := runRule(t, `async function f(x) { x = x + await Promise.resolve(1); }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestRequireAtomicUpdates_FlagsCompoundAssign(t *testing.T) {
	if n := runRule(t, `async function f(x) { x += await Promise.resolve(1); }`); n != 0 {
		// `x += await f()` has no x-identifier in the rhs, so it shouldn't fire.
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestRequireAtomicUpdates_FlagsReadInsideAwaitArg(t *testing.T) {
	if n := runRule(t, `async function f(x) { x = await g(x); }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestRequireAtomicUpdates_AllowsPlainAwait(t *testing.T) {
	if n := runRule(t, `async function f(x) { x = await g(); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestRequireAtomicUpdates_AllowsNonAwaitRead(t *testing.T) {
	if n := runRule(t, `function f(x) { x = x + 1; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestRequireAtomicUpdates_FlagsYieldRead(t *testing.T) {
	if n := runRule(t, `function* f(x) { x = x + (yield 1); }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestRequireAtomicUpdates_IgnoresNestedFunction(t *testing.T) {
	if n := runRule(t, `async function f(x) { x = (() => x)(); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
