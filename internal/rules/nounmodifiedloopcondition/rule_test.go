package nounmodifiedloopcondition_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounmodifiedloopcondition"
)

const tsconfigBody = `{"compilerOptions":{"strict":false,"target":"es2022","module":"esnext","moduleResolution":"bundler","allowJs":true,"noImplicitAny":false}}`

func runRule(t *testing.T, code string) int {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigBody), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(code), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nounmodifiedloopcondition.New()},
		map[string]wrapperlint.Severity{"no-unmodified-loop-condition": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUnmodifiedLoopCondition_FlagsUnchangedConditionVariable(t *testing.T) {
	if n := runRule(t, `var foo = 0; while (foo) { } foo = 1;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnmodifiedLoopCondition_AllowsConditionVariableUpdatedInBody(t *testing.T) {
	if n := runRule(t, `var foo = 0; while (foo) { ++foo; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnmodifiedLoopCondition_AllowsConditionWithSideEffectCall(t *testing.T) {
	if n := runRule(t, `var foo = 0; while (ok(foo)) { }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnmodifiedLoopCondition_AllowsConditionWithAssignment(t *testing.T) {
	if n := runRule(t, `var foo = 0; while (foo = next()) { }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
