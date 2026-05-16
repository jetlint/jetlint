package noemptycharacterclass_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noemptycharacterclass"
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
		[]engine.Rule{noemptycharacterclass.New()},
		map[string]wrapperlint.Severity{"no-empty-character-class": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoEmptyCharacterClass_FlagsEmptyClass(t *testing.T) {
	if n := runRule(t, `var r = /abc[]def/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoEmptyCharacterClass_FlagsEmptyClassAtStart(t *testing.T) {
	if n := runRule(t, `var r = /[]abc/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoEmptyCharacterClass_AllowsNonEmptyClass(t *testing.T) {
	if n := runRule(t, `var r = /[a-z]/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoEmptyCharacterClass_AllowsLiteralBracketsViaEscape(t *testing.T) {
	if n := runRule(t, `var r = /\[\]/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoEmptyCharacterClass_AllowsClassWithSingleChar(t *testing.T) {
	if n := runRule(t, `var r = /[a]/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoEmptyCharacterClass_HandlesFlags(t *testing.T) {
	if n := runRule(t, `var r = /abc[]def/gi;`); n != 1 {
		t.Errorf("expected 1 diagnostic with flags, got %d", n)
	}
}

func TestNoEmptyCharacterClass_AllowsNegatedClass(t *testing.T) {
	if n := runRule(t, `var r = /[^a]/;`); n != 0 {
		t.Errorf("expected 0 diagnostics on negated class, got %d", n)
	}
}
