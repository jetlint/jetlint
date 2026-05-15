package noselfassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noselfassign"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-nsa")
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
		[]engine.Rule{noselfassign.New()},
		map[string]wrapperlint.Severity{"no-self-assign": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoSelfAssign_FlagsIdentifierSelfAssign(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "let a = 1; a = a;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "no-self-assign" {
		t.Errorf("rule id = %q; want no-self-assign", diags[0].RuleID)
	}
}

func TestNoSelfAssign_FlagsMemberAccessSelfAssign(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const obj: { foo: number } = { foo: 1 }; obj.foo = obj.foo;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoSelfAssign_FlagsElementAccessSelfAssign(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "const arr: number[] = [1, 2]; arr[0] = arr[0];\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoSelfAssign_DoesNotFlagDifferentTargetAndSource(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "let a = 1; let b = 2; a = b;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestNoSelfAssign_DoesNotFlagArithmeticCompoundAssignments(t *testing.T) {
	// `+=`, `-=`, `*=`, `&=`, `|=` etc. change the stored value even
	// when both operands are the same (`a += a` is `a*2`), so the
	// rule stays silent on those.
	tsconfig := fixture(t, map[string]string{
		"main.ts": "let a = 1; a += a; a -= a; a *= a; a &= a; a |= a;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("arithmetic compound assignments must not be flagged; got %d diagnostics: %#v", len(diags), diags)
	}
}

func TestNoSelfAssign_FlagsLogicalCompoundAssignments(t *testing.T) {
	// `&&=`, `||=`, `??=` short-circuit, so `a ||= a`, `a &&= a`,
	// `a ??= a` are all no-ops and the rule flags them — matching
	// ESLint and oxlint.
	tsconfig := fixture(t, map[string]string{
		"main.ts": "let a: any = 1; a ||= a; a &&= a; a ??= a;\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics for ||= &&= ??=; got %d: %#v", len(diags), diags)
	}
}

func TestNoSelfAssign_DoesNotFlagDeclarationsWithSelfNamedInitializer(t *testing.T) {
	// `const x = x;` is a declaration, not an assignment; out of scope.
	// (It's also a TDZ runtime error, caught by other rules / TS.)
	tsconfig := fixture(t, map[string]string{
		"main.ts": "function f() { const obj: { x: number } = { x: 1 }; const x = obj.x; return x; }\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestNoSelfAssign_FlagsMultipleSelfAssignsInOneFile(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
let a = 1, b = 2, c = 3;
a = a;
b = b;
c = c;
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d", len(diags))
	}
}
