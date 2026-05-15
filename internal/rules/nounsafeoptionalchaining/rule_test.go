package nounsafeoptionalchaining_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounsafeoptionalchaining"
)

func runRule(t *testing.T, code string, rule engine.Rule) int {
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
		[]engine.Rule{rule},
		map[string]wrapperlint.Severity{"no-unsafe-optional-chaining": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUnsafeOptionalChaining_AllowsOptionalCall(t *testing.T) {
	if n := runRule(t, `obj?.foo();`, nounsafeoptionalchaining.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnsafeOptionalChaining_FlagsParenAccessOnChain(t *testing.T) {
	if n := runRule(t, `(obj?.foo).bar`, nounsafeoptionalchaining.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnsafeOptionalChaining_FlagsParenCallOnChain(t *testing.T) {
	if n := runRule(t, `(obj?.foo)();`, nounsafeoptionalchaining.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnsafeOptionalChaining_FlagsNewOnChain(t *testing.T) {
	if n := runRule(t, `new (obj?.foo)();`, nounsafeoptionalchaining.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnsafeOptionalChaining_FlagsDestructuringFromChain(t *testing.T) {
	if n := runRule(t, `const {foo} = obj?.bar;`, nounsafeoptionalchaining.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnsafeOptionalChaining_FlagsArithmeticUnderOption(t *testing.T) {
	rule := nounsafeoptionalchaining.NewWithOptions(nounsafeoptionalchaining.Options{DisallowArithmeticOperators: true})
	if n := runRule(t, `bar + obj?.foo;`, rule); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnsafeOptionalChaining_AllowsArithmeticByDefault(t *testing.T) {
	if n := runRule(t, `bar + obj?.foo;`, nounsafeoptionalchaining.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
