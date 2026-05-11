package nomisusedpromises_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nomisusedpromises"
)

func fixture(t *testing.T, source string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`),
		0o644); err != nil {
		t.Fatalf("tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(source), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return filepath.Join(dir, "tsconfig.json")
}

func runRule(t *testing.T, tsconfig string) []wrapperlint.Diagnostic {
	t.Helper()
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nomisusedpromises.New()},
		map[string]wrapperlint.Severity{"no-misused-promises": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoMisusedPromises_FlagsAsyncCallbackPassedToVoidConsumer(t *testing.T) {
	tsconfig := fixture(t, "const items = [1, 2, 3];\nitems.forEach(async (x) => { return x; });\n")
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "no-misused-promises" {
		t.Errorf("expected rule id 'no-misused-promises', got %q", diags[0].RuleID)
	}
}

func TestNoMisusedPromises_DoesNotFlagAsyncCallbackPassedToPromiseReturningConsumer(t *testing.T) {
	tsconfig := fixture(t, "const items = [1, 2, 3];\nconst doubled = items.map(async (x) => { return x * 2; });\n")
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for map (which accepts promise return), got %d: %#v", len(diags), diags)
	}
}
