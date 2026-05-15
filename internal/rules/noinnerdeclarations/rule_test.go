package noinnerdeclarations_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noinnerdeclarations"
)

func runRule(t *testing.T, code string, rule engine.Rule) int {
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
		[]engine.Rule{rule},
		map[string]wrapperlint.Severity{"no-inner-declarations": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoInnerDeclarations_FlagsFunctionInIf(t *testing.T) {
	rule := noinnerdeclarations.New()
	if n := runRule(t, `if (test) { function doSomething() { } }`, rule); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoInnerDeclarations_AllowsFunctionAtTopLevel(t *testing.T) {
	rule := noinnerdeclarations.New()
	if n := runRule(t, `function doSomething() { }`, rule); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoInnerDeclarations_AllowsFunctionInsideAnotherFunction(t *testing.T) {
	rule := noinnerdeclarations.New()
	if n := runRule(t, `function doSomething() { function somethingElse() { } }`, rule); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoInnerDeclarations_FlagsVarInIfWhenBothMode(t *testing.T) {
	rule := noinnerdeclarations.NewWithOptions(noinnerdeclarations.Options{Mode: noinnerdeclarations.ModeBoth})
	if n := runRule(t, `if (test) { var foo; }`, rule); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoInnerDeclarations_AllowsLetInIfWhenBothMode(t *testing.T) {
	rule := noinnerdeclarations.NewWithOptions(noinnerdeclarations.Options{Mode: noinnerdeclarations.ModeBoth})
	if n := runRule(t, `if (test) { let foo = 1; }`, rule); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
