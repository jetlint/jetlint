package nolossofprecision_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nolossofprecision"
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
		[]engine.Rule{nolossofprecision.New()},
		map[string]wrapperlint.Severity{"no-loss-of-precision": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoLossOfPrecision_FlagsSafeIntegerOverflow(t *testing.T) {
	if n := runRule(t, `var x = 9007199254740993;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoLossOfPrecision_AllowsSmallInteger(t *testing.T) {
	if n := runRule(t, `var x = 12345;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoLossOfPrecision_AllowsTrailingFractionalZeros(t *testing.T) {
	if n := runRule(t, `var x = 123.0000000000000000000000;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoLossOfPrecision_FlagsLargeBinary(t *testing.T) {
	if n := runRule(t, `var x = 0b100000000000000000000000000000000000000000000000000001;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoLossOfPrecision_AllowsSafeHex(t *testing.T) {
	if n := runRule(t, `var x = 0x1FFFFFFFFFFFFF;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
