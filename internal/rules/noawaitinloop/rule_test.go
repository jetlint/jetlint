package noawaitinloop_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noawaitinloop"
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
		[]engine.Rule{noawaitinloop.New()},
		map[string]wrapperlint.Severity{"no-await-in-loop": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoAwaitInLoop_FlagsAwaitInWhileBody(t *testing.T) {
	if n := runRule(t, `async function f() { while (true) { await x; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAwaitInLoop_FlagsAwaitInForOfBody(t *testing.T) {
	if n := runRule(t, `async function f() { for (const x of xs) { await x; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAwaitInLoop_AllowsAwaitInForOfIterable(t *testing.T) {
	if n := runRule(t, `async function f() { for (const x of await xs) {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAwaitInLoop_AllowsAwaitInsideForAwaitOf(t *testing.T) {
	// `for await...of` already serialises by design; an inner await is intended.
	if n := runRule(t, `async function f() { for await (const x of xs) { await x; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAwaitInLoop_AllowsAwaitInsideNestedAsyncFunction(t *testing.T) {
	if n := runRule(t, `async function f() { while (true) { async function g() { await x; } } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAwaitInLoop_AllowsAwaitInForStatementInitializer(t *testing.T) {
	if n := runRule(t, `async function f() { for (let i = await n; i < 10; i++) {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAwaitInLoop_FlagsAwaitInForStatementUpdate(t *testing.T) {
	if n := runRule(t, `async function f() { for (let i = 0; i < 10; i = await next()) {} }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAwaitInLoop_FlagsForAwaitOfInsideOuterLoop(t *testing.T) {
	if n := runRule(t, `async function f() { while (true) { for await (const x of xs) {} } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAwaitInLoop_FlagsAwaitUsingInLoopBody(t *testing.T) {
	if n := runRule(t, `async function f() { while (true) { await using r = getResource(); } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAwaitInLoop_AllowsAwaitUsingInForStatementInitializer(t *testing.T) {
	if n := runRule(t, `for (await using r = getResource(); ;) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
