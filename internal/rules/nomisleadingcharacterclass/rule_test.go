package nomisleadingcharacterclass_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nomisleadingcharacterclass"
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
		[]engine.Rule{nomisleadingcharacterclass.New()},
		map[string]wrapperlint.Severity{"no-misleading-character-class": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoMisleadingCharacterClass_FlagsLiteralAstralCodepoint(t *testing.T) {
	if n := runRule(t, "var r = /[🌷]/;"); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoMisleadingCharacterClass_AllowsAstralWithUnicodeFlag(t *testing.T) {
	if n := runRule(t, "var r = /[🌷]/u;"); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoMisleadingCharacterClass_AllowsAstralWithVFlag(t *testing.T) {
	if n := runRule(t, "var r = /[🌷]/v;"); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoMisleadingCharacterClass_FlagsSurrogatePairEscape(t *testing.T) {
	if n := runRule(t, `var r = /[🌷]/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoMisleadingCharacterClass_AllowsBMPCharacters(t *testing.T) {
	if n := runRule(t, `var r = /[a-z]/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoMisleadingCharacterClass_AllowsBMPUnicodeEscape(t *testing.T) {
	if n := runRule(t, `var r = /[ÿ]/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
