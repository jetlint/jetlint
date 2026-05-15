package getterreturn_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/getterreturn"
)

func runRule(t *testing.T, code string, opts getterreturn.Options) int {
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
		[]engine.Rule{getterreturn.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"getter-return": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestGetterReturn_AcceptsGetterReturningValue(t *testing.T) {
	if n := runRule(t, `var foo = { get bar() { return 1; } };`, getterreturn.Options{}); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestGetterReturn_FlagsEmptyGetter(t *testing.T) {
	if n := runRule(t, `var foo = { get bar() {} };`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_FlagsBareReturnByDefault(t *testing.T) {
	if n := runRule(t, `var foo = { get bar() { return; } };`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_AcceptsBareReturnWithAllowImplicit(t *testing.T) {
	if n := runRule(t, `var foo = { get bar() { return; } };`, getterreturn.Options{AllowImplicit: true}); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestGetterReturn_AcceptsClassGetterReturningValue(t *testing.T) {
	if n := runRule(t, `class Foo { get bar() { return 1; } }`, getterreturn.Options{}); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestGetterReturn_FlagsClassGetterMissingReturn(t *testing.T) {
	if n := runRule(t, `class Foo { get bar() {} }`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_FlagsObjectDefinePropertyGetterMissingReturn(t *testing.T) {
	if n := runRule(t, `Object.defineProperty(foo, "bar", { get: function() {} });`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_AcceptsObjectDefinePropertyGetterReturningValue(t *testing.T) {
	if n := runRule(t, `Object.defineProperty(foo, "bar", { get: function() { return 1; } });`, getterreturn.Options{}); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestGetterReturn_FlagsReflectDefinePropertyGetterMissingReturn(t *testing.T) {
	if n := runRule(t, `Reflect.defineProperty(foo, "bar", { get: function() {} });`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_FlagsObjectDefinePropertiesNestedGetter(t *testing.T) {
	if n := runRule(t, `Object.defineProperties(foo, { bar: { get: function() {} } });`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_FlagsObjectCreateNestedGetter(t *testing.T) {
	if n := runRule(t, `Object.create(foo, { bar: { get: function() {} } });`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_FlagsConditionalMissingReturn(t *testing.T) {
	if n := runRule(t, `var foo = { get bar() { if (x) return 1; } };`, getterreturn.Options{}); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestGetterReturn_AcceptsIfElseAllReturning(t *testing.T) {
	if n := runRule(t, `var foo = { get bar() { if (x) return 1; else return 2; } };`, getterreturn.Options{}); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestGetterReturn_IgnoresNonGetterProperty(t *testing.T) {
	if n := runRule(t, `var foo = { bar() {} };`, getterreturn.Options{}); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestGetterReturn_IgnoresSetterInDefineProperty(t *testing.T) {
	if n := runRule(t, `Object.defineProperty(foo, "bar", { set: function() {} });`, getterreturn.Options{}); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
