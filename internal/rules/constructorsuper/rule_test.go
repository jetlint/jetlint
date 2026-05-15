package constructorsuper_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/constructorsuper"
)

func runRule(t *testing.T, code string) []wrapperlint.Diagnostic {
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
		[]engine.Rule{constructorsuper.New()},
		map[string]wrapperlint.Severity{"constructor-super": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestConstructorSuper_AcceptsDerivedConstructorThatCallsSuper(t *testing.T) {
	diags := runRule(t, `class A extends B { constructor() { super(); } }`)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestConstructorSuper_FlagsDerivedConstructorWithoutSuper(t *testing.T) {
	diags := runRule(t, `class A extends B { constructor() {} }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "must call super") {
		t.Errorf("unexpected message: %q", diags[0].Message)
	}
}

func TestConstructorSuper_FlagsSuperInNonDerivedConstructor(t *testing.T) {
	diags := runRule(t, `class A { constructor() { super(); } }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "non-derived") {
		t.Errorf("message should mention non-derived; got %q", diags[0].Message)
	}
}

func TestConstructorSuper_FlagsSuperWhenExtendingLiteral(t *testing.T) {
	diags := runRule(t, `class A extends 5 { constructor() { super(); } }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestConstructorSuper_FlagsSuperWhenExtendingNull(t *testing.T) {
	diags := runRule(t, `class A extends null { constructor() { super(); } }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestConstructorSuper_AcceptsExtendsNullWithExplicitReturnValue(t *testing.T) {
	diags := runRule(t, `class A extends null { constructor() { return {}; } }`)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestConstructorSuper_FlagsExtendsNullWithoutSuperOrReturnValue(t *testing.T) {
	diags := runRule(t, `class A extends null { constructor() {} }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestConstructorSuper_FlagsSuperMissingOnSomePathViaBareReturn(t *testing.T) {
	diags := runRule(t, `class A extends B { constructor() { if (a) return; super(); } }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "every path") {
		t.Errorf("message should mention every path; got %q", diags[0].Message)
	}
}

func TestConstructorSuper_AcceptsEarlyThrowBeforeSuper(t *testing.T) {
	diags := runRule(t, `class A extends B { constructor() { if (a) throw new Error(); super(); } }`)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestConstructorSuper_AcceptsEarlyReturnWithValueBeforeSuper(t *testing.T) {
	diags := runRule(t, `class A extends B { constructor() { if (a) return {}; super(); } }`)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestConstructorSuper_FlagsDuplicateSuperCall(t *testing.T) {
	diags := runRule(t, `class A extends B { constructor() { super(); super(); } }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "more than once") {
		t.Errorf("message should mention duplicate; got %q", diags[0].Message)
	}
}

func TestConstructorSuper_AcceptsTernarySuper(t *testing.T) {
	diags := runRule(t, `class A extends B { constructor() { a ? super() : super(); } }`)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestConstructorSuper_AcceptsExtendsLogicalOr(t *testing.T) {
	diags := runRule(t, `class A extends (B || C) { constructor() { super(); } }`)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestConstructorSuper_FlagsExtendsLogicalAndWithUnconstructableRight(t *testing.T) {
	diags := runRule(t, `class A extends (B && 5) { constructor() { super(); } }`)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestConstructorSuper_IgnoresNestedClassConstructor(t *testing.T) {
	// Inner constructor's super belongs to inner class; outer class A
	// has no super call but extends nothing, which is fine.
	diags := runRule(t, `class A { constructor() { class B extends C { constructor() { super(); } } } }`)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestConstructorSuper_AcceptsForLoopWithEarlyReturnAfterSuper(t *testing.T) {
	code := `
class A extends Base {
    constructor(list) {
        for (const a of list) {
            if (a.foo) {
                super(a);
                return;
            }
        }
        super();
    }
}
`
	diags := runRule(t, code)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}
