package noclassassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noclassassign"
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
		[]engine.Rule{noclassassign.New()},
		map[string]wrapperlint.Severity{"no-class-assign": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoClassAssign_FlagsDirectReassignment(t *testing.T) {
	if n := runRule(t, `class A {} A = 0;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoClassAssign_FlagsReassignmentBeforeDeclaration(t *testing.T) {
	if n := runRule(t, `A = 0; class A {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoClassAssign_FlagsReassignmentFromMethodBody(t *testing.T) {
	if n := runRule(t, `class A { b() { A = 0; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoClassAssign_FlagsShorthandDestructure(t *testing.T) {
	if n := runRule(t, `class A {} ({A} = 0);`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoClassAssign_FlagsRenamedDestructureWithDefault(t *testing.T) {
	if n := runRule(t, `class A {} ({b: A = 0} = {});`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoClassAssign_FlagsNamedClassExpressionReassignmentFromMethod(t *testing.T) {
	if n := runRule(t, `let A = class A { b() { A = 0; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoClassAssign_IgnoresParameterShadowing(t *testing.T) {
	if n := runRule(t, `class A { b(A: any) { A = 0; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoClassAssign_IgnoresLocalShadowing(t *testing.T) {
	if n := runRule(t, `class A { b() { let A; A = 0; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoClassAssign_IgnoresLetReassignmentWhenClassExpressionDiffersInName(t *testing.T) {
	// `A` outside the class refers to the mutable let, not the class.
	if n := runRule(t, `let A = class B {}; A = 1;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoClassAssign_IgnoresReadingTheClassBinding(t *testing.T) {
	if n := runRule(t, `class A {} foo(A);`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
