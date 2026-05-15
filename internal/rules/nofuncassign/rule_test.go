package nofuncassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nofuncassign"
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
		[]engine.Rule{nofuncassign.New()},
		map[string]wrapperlint.Severity{"no-func-assign": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoFuncAssign_FlagsAssignmentToDeclaredFunction(t *testing.T) {
	if n := runRule(t, `function foo() {}; foo = bar;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoFuncAssign_FlagsAssignmentInsideDeclaredFunction(t *testing.T) {
	if n := runRule(t, `function foo() { foo = bar; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoFuncAssign_AllowsVarFunctionExpressionReassignment(t *testing.T) {
	if n := runRule(t, `var foo = function() {}; foo = bar;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoFuncAssign_FlagsNamedFunctionExpressionSelfAssignment(t *testing.T) {
	if n := runRule(t, `var a = function foo() { foo = 123; };`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoFuncAssign_AllowsShadowedParameter(t *testing.T) {
	if n := runRule(t, `function foo(foo) { foo = bar; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
