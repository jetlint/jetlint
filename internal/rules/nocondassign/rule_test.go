package nocondassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nocondassign"
)

func runRule(t *testing.T, code string, opts nocondassign.Options) int {
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
		[]engine.Rule{nocondassign.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-cond-assign": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

var defaultOpts = nocondassign.Options{Mode: nocondassign.ModeExceptParens}
var alwaysOpts = nocondassign.Options{Mode: nocondassign.ModeAlways}

func TestNoCondAssign_FlagsBareAssignmentInIfTest(t *testing.T) {
	if n := runRule(t, `if (a = b) {}`, defaultOpts); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCondAssign_FlagsBareAssignmentInWhileTest(t *testing.T) {
	if n := runRule(t, `while (a = b) {}`, defaultOpts); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCondAssign_AllowsParenWrappedAssignmentInDefaultMode(t *testing.T) {
	if n := runRule(t, `if ((a = b)) {}`, defaultOpts); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCondAssign_FlagsParenWrappedAssignmentInAlwaysMode(t *testing.T) {
	if n := runRule(t, `if ((a = b)) {}`, alwaysOpts); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCondAssign_AllowsEqualityComparison(t *testing.T) {
	if n := runRule(t, `if (a == b) {}`, defaultOpts); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCondAssign_AllowsAssignmentInsideIfBody(t *testing.T) {
	if n := runRule(t, `if (cond) { a = b; }`, alwaysOpts); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCondAssign_FlagsConditionalExpressionTestEvenWithParensInDefaultMode(t *testing.T) {
	if n := runRule(t, `var b = (x = 0) ? 1 : 0;`, defaultOpts); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCondAssign_FlagsAssignmentBuriedInWhileTestUnderAlwaysMode(t *testing.T) {
	if n := runRule(t, `while (a || (b = c)) {}`, alwaysOpts); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCondAssign_AllowsAssignmentBuriedInWhileTestUnderDefaultMode(t *testing.T) {
	if n := runRule(t, `while (a || (b = c)) {}`, defaultOpts); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCondAssign_AllowsAssignmentInsideNestedFunctionInAlwaysMode(t *testing.T) {
	if n := runRule(t, `if ((function() { return a = b; })()) {}`, alwaysOpts); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
