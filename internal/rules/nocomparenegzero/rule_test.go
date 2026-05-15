package nocomparenegzero_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nocomparenegzero"
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
		[]engine.Rule{nocomparenegzero.New()},
		map[string]wrapperlint.Severity{"no-compare-neg-zero": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoCompareNegZero_FlagsStrictEqualityAgainstNegZero(t *testing.T) {
	if n := runRule(t, `x === -0;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCompareNegZero_FlagsNegZeroOnLeftOfGreaterThan(t *testing.T) {
	if n := runRule(t, `-0 > x;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCompareNegZero_FlagsBigIntNegZero(t *testing.T) {
	if n := runRule(t, `x === -0n;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoCompareNegZero_AllowsComparisonAgainstPositiveZero(t *testing.T) {
	if n := runRule(t, `x === 0;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCompareNegZero_AllowsComparisonAgainstStringNegZero(t *testing.T) {
	if n := runRule(t, `x === '-0';`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCompareNegZero_AllowsComparisonAgainstNegOne(t *testing.T) {
	if n := runRule(t, `x === -1;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCompareNegZero_AllowsAssignmentToNegZero(t *testing.T) {
	// Only comparison operators trigger; the rule does not flag arithmetic.
	if n := runRule(t, `let x = -0;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoCompareNegZero_FlagsParenthesizedNegZero(t *testing.T) {
	if n := runRule(t, `x === (-0);`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
