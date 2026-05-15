package noemptypattern_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noemptypattern"
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
		[]engine.Rule{noemptypattern.New()},
		map[string]wrapperlint.Severity{"no-empty-pattern": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoEmptyPattern_FlagsEmptyObject(t *testing.T) {
	if n := runRule(t, `var {} = foo;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoEmptyPattern_FlagsEmptyArray(t *testing.T) {
	if n := runRule(t, `var [] = foo;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoEmptyPattern_FlagsNestedEmptyObject(t *testing.T) {
	if n := runRule(t, `var {a: {}} = foo;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoEmptyPattern_AllowsDefaultValuePattern(t *testing.T) {
	if n := runRule(t, `var {a = {}} = foo;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoEmptyPattern_FlagsEmptyParameterPattern(t *testing.T) {
	if n := runRule(t, `function foo({}) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
