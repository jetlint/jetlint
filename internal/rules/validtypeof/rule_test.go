package validtypeof_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/validtypeof"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-vto")
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
		[]engine.Rule{validtypeof.New()},
		map[string]wrapperlint.Severity{"valid-typeof": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestValidTypeof_FlagsTypoedTypeofString(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: unknown = 1; if (typeof x === \"strnig\") { /* noop */ }\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "valid-typeof" {
		t.Errorf("rule id = %q; want valid-typeof", diags[0].RuleID)
	}
	if !strings.Contains(diags[0].Message, "strnig") {
		t.Errorf("message should name the invalid literal; got %q", diags[0].Message)
	}
}

func TestValidTypeof_AcceptsEveryValidTypeofResult(t *testing.T) {
	validStrings := []string{"undefined", "object", "boolean", "number", "string", "function", "symbol", "bigint"}
	for _, s := range validStrings {
		t.Run(s, func(t *testing.T) {
			tsconfig := fixture(t, map[string]string{
				"main.ts": "const x: unknown = 1; const r = typeof x === \"" + s + "\";\n",
			})
			diags := runRule(t, tsconfig)
			if len(diags) != 0 {
				t.Fatalf("valid string %q should not be flagged; got %d diagnostics: %#v", s, len(diags), diags)
			}
		})
	}
}

func TestValidTypeof_FlagsInvalidStringOnEitherSide(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: unknown = 1; const r1 = typeof x === \"undefned\"; const r2 = \"undefned\" === typeof x;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics (one per side), got %d", len(diags))
	}
}

func TestValidTypeof_FlagsAcrossAllEqualityOperators(t *testing.T) {
	for _, op := range []string{"===", "!==", "==", "!="} {
		t.Run(op, func(t *testing.T) {
			tsconfig := fixture(t, map[string]string{
				"main.ts": "const x: unknown = 1; const r = typeof x " + op + " \"bogus\";\n",
			})
			diags := runRule(t, tsconfig)
			if len(diags) != 1 {
				t.Fatalf("operator %q: expected 1 diagnostic, got %d", op, len(diags))
			}
		})
	}
}

func TestValidTypeof_DoesNotFlagRelationalOperators(t *testing.T) {
	// ESLint's valid-typeof only inspects equality operators; relational
	// comparisons (`<`, `>`, etc.) against typeof are nonsensical but
	// out of scope for this rule.
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: unknown = 1; const r = typeof x > \"bogus\";\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("relational operator should not be flagged; got %d diagnostics", len(diags))
	}
}

func TestValidTypeof_DoesNotFlagComparisonBetweenTwoTypeofs(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: unknown = 1; const y: unknown = 2; const r = typeof x === typeof y;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("typeof === typeof should not be flagged; got %d diagnostics", len(diags))
	}
}

func TestValidTypeof_DoesNotFlagComparisonAgainstVariable(t *testing.T) {
	// When the non-typeof side is not a string literal (it's a variable),
	// the rule can't statically determine validity. Don't flag.
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: unknown = 1; const want = \"strnig\"; const r = typeof x === want;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("non-literal comparison should not be flagged; got %d", len(diags))
	}
}

func TestValidTypeof_FlagsTemplateLiteralWithInvalidString(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: unknown = 1; const r = typeof x === `bogus`;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("template literal with invalid string should be flagged; got %d", len(diags))
	}
}

func TestValidTypeof_DoesNotFlagNonComparisonBinaryWithTypeof(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const x: unknown = 1; const r = typeof x + \"!\";\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("non-equality operator should not be flagged; got %d", len(diags))
	}
}
