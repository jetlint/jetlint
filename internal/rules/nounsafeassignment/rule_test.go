package nounsafeassignment_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounsafeassignment"
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

// Regression for jetlint#621: assigning an object literal to a typed
// interface binding caused the rule to panic on every such site (5,281
// rule-panic diagnostics across a real codebase). The trigger is a
// non-tuple TypeReference whose target is not InterfaceType-backed, on
// which the wrapper's TypeArguments() unsafely dispatches.
func TestNoUnsafeAssignment_DoesNotPanicWhenObjectLiteralIsAssignedToInterfaceTypeBinding(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "interface Config { content: string[] }\nconst c: Config = { content: ['x'] };\n",
	})
	diags := runRule(t, tsconfig)
	for _, d := range diags {
		if d.RuleID == "jetlint/rule-panic" {
			t.Fatalf("expected no rule-panic diagnostics, got: %#v", diags)
		}
	}
}

// Regression for jetlint#621: object-literal property values passed as
// call arguments (e.g., `f({ start: value })`) and JSX prop value
// expressions (e.g., `<C selected={value} />`) panicked when the
// rule probed contextual type arguments.
func TestNoUnsafeAssignment_DoesNotPanicOnObjectLiteralPropertyValuesInCallArguments(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "declare function f(i: { start: Date | number; end: Date | number }): void;\nconst sd: Date = new Date();\nconst ed: Date = new Date();\nf({ start: sd, end: ed });\n",
	})
	diags := runRule(t, tsconfig)
	for _, d := range diags {
		if d.RuleID == "jetlint/rule-panic" {
			t.Fatalf("expected no rule-panic diagnostics, got: %#v", diags)
		}
	}
}
