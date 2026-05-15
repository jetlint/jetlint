package noobjcalls_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noobjcalls"
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
		[]engine.Rule{noobjcalls.New()},
		map[string]wrapperlint.Severity{"no-obj-calls": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoObjCalls_FlagsMathCall(t *testing.T) {
	if n := runRule(t, `Math();`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoObjCalls_FlagsNewJSON(t *testing.T) {
	if n := runRule(t, `new JSON();`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoObjCalls_AllowsMemberAccess(t *testing.T) {
	if n := runRule(t, `Math.random();`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoObjCalls_AllowsShadowedBinding(t *testing.T) {
	if n := runRule(t, `var Math; Math();`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoObjCalls_FlagsGlobalThisMember(t *testing.T) {
	if n := runRule(t, `globalThis.Math();`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoObjCalls_FlagsAliasChain(t *testing.T) {
	if n := runRule(t, `let a = JSON; let b = a; b();`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
