package nonewnativenonconstructor_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nonewnativenonconstructor"
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
		[]engine.Rule{nonewnativenonconstructor.New()},
		map[string]wrapperlint.Severity{"no-new-native-nonconstructor": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoNewNativeNonconstructor_FlagsNewSymbol(t *testing.T) {
	if n := runRule(t, `var foo = new Symbol('foo');`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoNewNativeNonconstructor_FlagsNewBigInt(t *testing.T) {
	if n := runRule(t, `var foo = new BigInt(1);`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoNewNativeNonconstructor_AllowsBareCall(t *testing.T) {
	if n := runRule(t, `var foo = Symbol('foo');`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoNewNativeNonconstructor_AllowsShadowedSymbol(t *testing.T) {
	if n := runRule(t, `function Symbol() {} new Symbol();`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoNewNativeNonconstructor_AllowsParameterShadow(t *testing.T) {
	if n := runRule(t, `function bar(Symbol) { var baz = new Symbol('baz'); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
