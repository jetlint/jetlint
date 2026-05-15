package useisnan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/useisnan"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-uin")
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
		[]engine.Rule{useisnan.New()},
		map[string]wrapperlint.Severity{"use-isnan": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestUseIsNaN_FlagsComparisonAgainstNaNIdentifier(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x = 1; if (x === NaN) { /* noop */ }\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "use-isnan" {
		t.Errorf("rule id = %q; want use-isnan", diags[0].RuleID)
	}
	if !strings.Contains(diags[0].Message, "isNaN") {
		t.Errorf("message should mention isNaN; got %q", diags[0].Message)
	}
}

func TestUseIsNaN_FlagsNaNOnEitherSide(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x = 1; const r1 = x === NaN; const r2 = NaN === x;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}
}

func TestUseIsNaN_FlagsEveryComparisonOperator(t *testing.T) {
	operators := []string{"===", "!==", "==", "!=", ">", ">=", "<", "<="}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			tsconfig := fixture(t, map[string]string{
				"main.ts": "const x = 1; const r = (x " + op + " NaN);\n",
			})
			diags := runRule(t, tsconfig)
			if len(diags) != 1 {
				t.Fatalf("operator %q: expected 1 diagnostic, got %d", op, len(diags))
			}
		})
	}
}

func TestUseIsNaN_FlagsNumberNaNPropertyAccess(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x = 1; const r = x === Number.NaN;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
}

func TestUseIsNaN_FlagsParenthesizedNaN(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x = 1; const r = x === (NaN);\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestUseIsNaN_DoesNotFlagComparisonsWithoutNaN(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x = 1; const y = 2; const r = x === y; if (x > 5) { /* noop */ }\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestUseIsNaN_DoesNotFlagNonComparisonOperators(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x = NaN + 1; const y = NaN * 2; const z = NaN && 3;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestUseIsNaN_DoesNotFlagLocalVariableNamedNaN(t *testing.T) {
	// Even if a user shadows NaN, lexical analysis only looks at the
	// name; this is acceptable for v1 since shadowing NaN is itself a
	// pathological pattern users should not write.
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const NaN_value = 5; const x = 1; const r = x === NaN_value;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for NaN_value (not NaN), got %d", len(diags))
	}
}

func TestUseIsNaN_FlagsBothSidesAreNaN(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const r = NaN === NaN;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic (one binary expression), got %d", len(diags))
	}
}
