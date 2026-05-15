package noselfcompare_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noselfcompare"
)

// fixture writes a TypeScript project to a temp dir and returns the
// absolute tsconfig path. A minimal tsconfig is supplied unless the
// caller overrides it.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-nsc")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if _, hasConfig := files["tsconfig.json"]; !hasConfig {
		files["tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
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
		[]engine.Rule{noselfcompare.New()},
		map[string]wrapperlint.Severity{"no-self-compare": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoSelfCompare_FlagsIdenticalIdentifierComparison(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const a = 1; if (a === a) { /* noop */ }\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "no-self-compare" {
		t.Errorf("rule id = %q; want no-self-compare", diags[0].RuleID)
	}
	if !strings.Contains(diags[0].Message, "itself") {
		t.Errorf("message should mention comparing to itself; got %q", diags[0].Message)
	}
}

func TestNoSelfCompare_FlagsEveryComparisonOperator(t *testing.T) {
	operators := []string{"===", "!==", "==", "!=", ">", ">=", "<", "<="}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			tsconfig := fixture(t, map[string]string{
				"main.ts": "const a = 1; const r = (a " + op + " a);\n",
			})
			diags := runRule(t, tsconfig)
			if len(diags) != 1 {
				t.Fatalf("operator %q: expected 1 diagnostic, got %d", op, len(diags))
			}
		})
	}
}

func TestNoSelfCompare_FlagsMemberAccessComparison(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const obj = { foo: 1 }; const r = obj.foo === obj.foo;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoSelfCompare_FlagsElementAccessComparison(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const arr = [1, 2, 3]; const r = arr[0] === arr[0];\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoSelfCompare_DoesNotFlagDifferentOperands(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const a = 1; const b = 2; const r1 = a === b; const r2 = a !== b;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestNoSelfCompare_DoesNotFlagDifferentMemberAccess(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const obj = { x: 1, y: 2 }; const r = obj.x === obj.y;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestNoSelfCompare_DoesNotFlagAssignmentOrLogicalOperators(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "let a = 1; a = a; const r1 = a && a; const r2 = a || a;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestNoSelfCompare_DoesNotFlagArithmeticOperators(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const a = 1; const r1 = a + a; const r2 = a - a; const r3 = a * a;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}
