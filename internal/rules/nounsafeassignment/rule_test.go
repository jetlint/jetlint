package nounsafeassignment_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/nounsafeassignment"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if _, has := files["tsconfig.json"]; !has {
		files["tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler","noImplicitAny":true}}`
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
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
		[]engine.Rule{nounsafeassignment.New()},
		map[string]wrapperlint.Severity{"no-unsafe-assignment": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoUnsafeAssignment_FlagsAssignmentFromAnyToTypedTarget(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "declare const dyn: any;\nconst x: number = dyn;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "no-unsafe-assignment" {
		t.Errorf("expected rule id 'no-unsafe-assignment', got %q", diags[0].RuleID)
	}
}

func TestNoUnsafeAssignment_DoesNotFlagWellTypedAssignment(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: number = 42;\nconst y: string = \"hi\";\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestNoUnsafeAssignment_DoesNotFlagAnyToAny(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "declare const dyn: any;\nconst x: any = dyn;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for any-to-any, got %d: %#v", len(diags), diags)
	}
}
