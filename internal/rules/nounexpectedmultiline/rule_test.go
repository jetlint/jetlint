package nounexpectedmultiline_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounexpectedmultiline"
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
		[]engine.Rule{nounexpectedmultiline.New()},
		map[string]wrapperlint.Severity{"no-unexpected-multiline": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUnexpectedMultiline_FlagsCallAfterNewline(t *testing.T) {
	if n := runRule(t, "var a = b\n(x).foo()"); n < 1 {
		t.Errorf("expected diagnostic, got %d", n)
	}
}

func TestNoUnexpectedMultiline_FlagsBracketAfterNewline(t *testing.T) {
	if n := runRule(t, "var a = b\n[1, 2].forEach(x => x)"); n < 1 {
		t.Errorf("expected diagnostic, got %d", n)
	}
}

func TestNoUnexpectedMultiline_FlagsTaggedTemplateAfterNewline(t *testing.T) {
	if n := runRule(t, "let x = function() {}\n`hello`"); n < 1 {
		t.Errorf("expected diagnostic, got %d", n)
	}
}

func TestNoUnexpectedMultiline_AllowsSameLineCall(t *testing.T) {
	if n := runRule(t, "var a = b; (x).foo()"); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnexpectedMultiline_AllowsOptionalChain(t *testing.T) {
	if n := runRule(t, "var a = b\n  ?.(x).foo()"); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
