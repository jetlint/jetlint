package fordirection_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/fordirection"
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
		[]engine.Rule{fordirection.New()},
		map[string]wrapperlint.Severity{"for-direction": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestForDirection_AcceptsAscendingLoop(t *testing.T) {
	if n := runRule(t, `for (let i=0; i<10; i++) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_AcceptsDescendingLoop(t *testing.T) {
	if n := runRule(t, `for (let i=10; i>0; i--) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_FlagsAscendingTestWithDecrement(t *testing.T) {
	if n := runRule(t, `for (let i=0; i<10; i--) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestForDirection_FlagsDescendingTestWithIncrement(t *testing.T) {
	if n := runRule(t, `for (let i=10; i>0; i++) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestForDirection_AcceptsCounterOnRight(t *testing.T) {
	if n := runRule(t, `for (let i=0; 10>i; i++) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_FlagsWrongDirectionWithCounterOnRight(t *testing.T) {
	if n := runRule(t, `for (let i=0; 10>i; i--) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestForDirection_AcceptsCompoundAssignmentForward(t *testing.T) {
	if n := runRule(t, `for (let i=0; i<10; i+=1) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_FlagsCompoundAssignmentBackward(t *testing.T) {
	if n := runRule(t, `for (let i=0; i<10; i-=1) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestForDirection_AcceptsCompoundAssignmentNegativeStepDescending(t *testing.T) {
	// i += -1 moves backward, matching descending test.
	if n := runRule(t, `for (let i=10; i>0; i+=-1) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_IgnoresUpdateOnDifferentVariable(t *testing.T) {
	if n := runRule(t, `for (let i=0; i<10; j++) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_IgnoresLoopWithoutCondition(t *testing.T) {
	if n := runRule(t, `for (;;) { break; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_IgnoresNonComparisonTest(t *testing.T) {
	if n := runRule(t, `for (let i=0; i; i++) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestForDirection_IgnoresUnknownStepValue(t *testing.T) {
	if n := runRule(t, `for (let i=0; i<10; i+=n) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
