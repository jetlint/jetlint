package nocontrolregex_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nocontrolregex"
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
		[]engine.Rule{nocontrolregex.New()},
		map[string]wrapperlint.Severity{"no-control-regex": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoControlRegex_FlagsHexEscapeBelowSpace(t *testing.T) {
	if n := runRule(t, `var r = /\x1f/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoControlRegex_FlagsUnicodeEscape(t *testing.T) {
	if n := runRule(t, `var r = //;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoControlRegex_FlagsUnicodeBraceEscape(t *testing.T) {
	if n := runRule(t, `var r = /\u{1f}/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoControlRegex_AllowsNonControlHex(t *testing.T) {
	if n := runRule(t, `var r = /\x20/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoControlRegex_AllowsRegularEscapes(t *testing.T) {
	if n := runRule(t, `var r = /\d\w\s/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoControlRegex_FlagsRegExpStringArg(t *testing.T) {
	if n := runRule(t, `var r = new RegExp("\\x1f");`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoControlRegex_AllowsRegExpWithNonControl(t *testing.T) {
	if n := runRule(t, `var r = new RegExp("abc");`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoControlRegex_FlagsLiteralControlChar(t *testing.T) {
	if n := runRule(t, "var r = /\x01/;"); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
