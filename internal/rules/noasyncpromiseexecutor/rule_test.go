package noasyncpromiseexecutor_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noasyncpromiseexecutor"
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
		[]engine.Rule{noasyncpromiseexecutor.New()},
		map[string]wrapperlint.Severity{"no-async-promise-executor": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoAsyncPromiseExecutor_AcceptsSyncArrowExecutor(t *testing.T) {
	if n := runRule(t, `new Promise((resolve, reject) => {})`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_AcceptsSyncFunctionExecutor(t *testing.T) {
	if n := runRule(t, `new Promise(function (resolve, reject) {})`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_IgnoresAsyncExecutorOnNonPromiseConstructor(t *testing.T) {
	if n := runRule(t, `new Foo(async (resolve, reject) => {})`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_FlagsAsyncArrowExecutor(t *testing.T) {
	if n := runRule(t, `new Promise(async (resolve, reject) => {})`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_FlagsAsyncFunctionExecutor(t *testing.T) {
	if n := runRule(t, `new Promise(async function foo(resolve, reject) {})`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_FlagsAsyncExecutorWrappedInParens(t *testing.T) {
	if n := runRule(t, `new Promise(((((async () => {})))))`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_IgnoresAsyncSecondArgument(t *testing.T) {
	// Only the executor (first argument) matters.
	if n := runRule(t, `new Promise((resolve, reject) => {}, async function unrelated() {})`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_IgnoresPromiseCallWithoutNew(t *testing.T) {
	if n := runRule(t, `Promise(async () => {})`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoAsyncPromiseExecutor_IgnoresNewPromiseWithoutArguments(t *testing.T) {
	if n := runRule(t, `new Promise()`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
