package nounusedprivateclassmembers_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounusedprivateclassmembers"
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
		[]engine.Rule{nounusedprivateclassmembers.New()},
		map[string]wrapperlint.Severity{"no-unused-private-class-members": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUnusedPrivateClassMembers_FlagsUnusedField(t *testing.T) {
	if n := runRule(t, `class A { #x = 1; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedPrivateClassMembers_AllowsReadField(t *testing.T) {
	if n := runRule(t, `class A { #x = 1; m() { return this.#x; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedPrivateClassMembers_FlagsWriteOnlyField(t *testing.T) {
	if n := runRule(t, `class A { #x = 1; m() { this.#x = 2; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedPrivateClassMembers_FlagsUpdateOnlyField(t *testing.T) {
	if n := runRule(t, `class A { #x = 1; m() { this.#x++; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedPrivateClassMembers_AllowsUpdateResultUsed(t *testing.T) {
	if n := runRule(t, `class A { #x = 1; m() { return this.#x++; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedPrivateClassMembers_FlagsUnusedMethod(t *testing.T) {
	if n := runRule(t, `class A { #m() {} }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedPrivateClassMembers_AllowsCalledMethod(t *testing.T) {
	if n := runRule(t, `class A { #m() {} p() { this.#m(); } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
