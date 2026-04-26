package strictbooleanexpressions_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/strictbooleanexpressions"
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
		t.Fatalf("write main.ts: %v", err)
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
		[]engine.Rule{strictbooleanexpressions.New()},
		map[string]wrapperlint.Severity{"strict-boolean-expressions": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestStrictBooleanExpressions_FlagsTestOfStringUndefinedUnion(t *testing.T) {
	tsconfig := fixture(t, `
declare const maybeName: string | undefined;
if (maybeName) { console.log(maybeName); }
`)
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "strict-boolean-expressions" {
		t.Errorf("expected rule id 'strict-boolean-expressions', got %q", diags[0].RuleID)
	}
}

func TestStrictBooleanExpressions_DoesNotFlagBooleanTest(t *testing.T) {
	tsconfig := fixture(t, `
declare const flag: boolean;
if (flag) { console.log("yes"); }
`)
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %d: %#v", len(diags), diags)
	}
}
