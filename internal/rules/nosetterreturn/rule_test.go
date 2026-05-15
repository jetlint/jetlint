package nosetterreturn_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nosetterreturn"
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
		[]engine.Rule{nosetterreturn.New()},
		map[string]wrapperlint.Severity{"no-setter-return": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoSetterReturn_FlagsClassSetterReturn(t *testing.T) {
	if n := runRule(t, `class C { set foo(v) { return 1; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoSetterReturn_FlagsObjectSetterReturn(t *testing.T) {
	if n := runRule(t, `({ set foo(v) { return 1; } });`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoSetterReturn_AllowsEmptyReturn(t *testing.T) {
	if n := runRule(t, `class C { set foo(v) { return; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoSetterReturn_AllowsReturnFromNestedFunction(t *testing.T) {
	if n := runRule(t, `class C { set foo(v) { (function() { return 1; })(); } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoSetterReturn_AllowsGetterReturn(t *testing.T) {
	if n := runRule(t, `class C { get foo() { return 1; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
