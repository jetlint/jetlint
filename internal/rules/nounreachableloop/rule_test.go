package nounreachableloop_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounreachableloop"
)

func runRule(t *testing.T, code string) int {
	t.Helper()
	dir := t.TempDir()
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(`{"compilerOptions":{"strict":false,"target":"es2022","module":"esnext","moduleResolution":"bundler","allowJs":true,"noImplicitAny":false}}`), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte("function _w(){\n"+code+"\n}"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nounreachableloop.New()},
		map[string]wrapperlint.Severity{"no-unreachable-loop": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUnreachableLoop_FlagsUnconditionalBreakInFor(t *testing.T) {
	if n := runRule(t, `for (let i = 0; i < 10; i++) { break; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnreachableLoop_FlagsUnconditionalReturnInWhile(t *testing.T) {
	if n := runRule(t, `while (true) { return 1; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnreachableLoop_FlagsThrowInDoWhile(t *testing.T) {
	if n := runRule(t, `do { throw new Error(); } while (true);`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnreachableLoop_FlagsBothBranchesExit(t *testing.T) {
	if n := runRule(t, `while (true) { if (x) return; else break; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnreachableLoop_AllowsNormalLoop(t *testing.T) {
	if n := runRule(t, `for (let i = 0; i < 10; i++) { doSomething(i); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnreachableLoop_AllowsContinue(t *testing.T) {
	if n := runRule(t, `while (true) { if (x) continue; doSomething(); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnreachableLoop_AllowsConditionalBreak(t *testing.T) {
	if n := runRule(t, `while (true) { if (done) break; work(); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnreachableLoop_AllowsForIn(t *testing.T) {
	if n := runRule(t, `for (const k in obj) { handle(k); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnreachableLoop_FlagsForInWithBreak(t *testing.T) {
	if n := runRule(t, `for (const k in obj) { break; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnreachableLoop_FlagsForOfWithReturn(t *testing.T) {
	if n := runRule(t, `for (const x of xs) { return x; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
