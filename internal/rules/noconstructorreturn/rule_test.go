package noconstructorreturn_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noconstructorreturn"
)

func runRule(t *testing.T, code string) int {
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
		[]engine.Rule{noconstructorreturn.New()},
		map[string]wrapperlint.Severity{"no-constructor-return": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoConstructorReturn_FlagsReturnWithValueFromConstructor(t *testing.T) {
	if n := runRule(t, `class C { constructor() { return ''; } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstructorReturn_AllowsBareReturnInConstructor(t *testing.T) {
	if n := runRule(t, `class C { constructor() { return; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstructorReturn_AllowsBareReturnInsideConditional(t *testing.T) {
	if n := runRule(t, `class C { constructor(a: any) { if (!a) { return; } else { a(); } } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstructorReturn_FlagsValueReturnInsideConditional(t *testing.T) {
	if n := runRule(t, `class C { constructor(a: any) { if (!a) { return ''; } else { a(); } } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstructorReturn_IgnoresReturnInNestedFunction(t *testing.T) {
	if n := runRule(t, `class C { constructor() { function fn() { return true; } } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstructorReturn_IgnoresReturnInNestedArrowFunction(t *testing.T) {
	if n := runRule(t, `class C { constructor() { this.fn = () => { return true; }; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstructorReturn_AllowsReturnInPlainMethod(t *testing.T) {
	if n := runRule(t, `class C { method() { return ''; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstructorReturn_AllowsReturnInGetter(t *testing.T) {
	if n := runRule(t, `class C { get v() { return ''; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstructorReturn_AllowsReturnInRegularFunction(t *testing.T) {
	if n := runRule(t, `function fn() { return 1; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
