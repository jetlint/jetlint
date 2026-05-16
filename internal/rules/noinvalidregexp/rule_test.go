package noinvalidregexp_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noinvalidregexp"
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
		[]engine.Rule{noinvalidregexp.New()},
		map[string]wrapperlint.Severity{"no-invalid-regexp": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoInvalidRegexp_FlagsUnmatchedOpenParenInRegExpCall(t *testing.T) {
	if n := runRule(t, `var r = new RegExp("(");`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoInvalidRegexp_FlagsUnmatchedCloseParenInRegExpCall(t *testing.T) {
	if n := runRule(t, `var r = new RegExp(")");`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoInvalidRegexp_FlagsUnclosedClassInRegExpCall(t *testing.T) {
	if n := runRule(t, `var r = new RegExp("[abc");`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoInvalidRegexp_AllowsValidPatternInRegExpCall(t *testing.T) {
	if n := runRule(t, `var r = new RegExp("(a|b)+");`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoInvalidRegexp_AllowsValidLiteral(t *testing.T) {
	if n := runRule(t, `var r = /(a|b)+/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoInvalidRegexp_FlagsUnterminatedNamedGroup(t *testing.T) {
	if n := runRule(t, `var r = new RegExp("(?<name");`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoInvalidRegexp_AllowsValidNamedGroup(t *testing.T) {
	if n := runRule(t, `var r = /(?<foo>a)/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
