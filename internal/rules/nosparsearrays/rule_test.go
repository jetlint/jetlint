package nosparsearrays_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nosparsearrays"
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
		[]engine.Rule{nosparsearrays.New()},
		map[string]wrapperlint.Severity{"no-sparse-arrays": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoSparseArrays_FlagsHole(t *testing.T) {
	if n := runRule(t, `var a = [1, , 2];`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoSparseArrays_AllowsTrailingComma(t *testing.T) {
	if n := runRule(t, `var a = [1, 2,];`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoSparseArrays_FlagsAllCommaArray(t *testing.T) {
	if n := runRule(t, `var a = [,];`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoSparseArrays_AllowsEmptyArray(t *testing.T) {
	if n := runRule(t, `var a = [];`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
