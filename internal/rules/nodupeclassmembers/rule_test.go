package nodupeclassmembers_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nodupeclassmembers"
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
		[]engine.Rule{nodupeclassmembers.New()},
		map[string]wrapperlint.Severity{"no-dupe-class-members": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoDupeClassMembers_FlagsDuplicateMethod(t *testing.T) {
	if n := runRule(t, `class A { foo() {} foo() {} }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDupeClassMembers_AllowsGetterSetterPair(t *testing.T) {
	if n := runRule(t, `class A { get foo() { return 0; } set foo(v: number) {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDupeClassMembers_AllowsSameNameDifferingStatic(t *testing.T) {
	if n := runRule(t, `class A { foo() {} static foo() {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDupeClassMembers_AllowsNonStaticConstructorAndNamedMethod(t *testing.T) {
	if n := runRule(t, `class A { ['constructor']() {} constructor() {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDupeClassMembers_FlagsDuplicateStaticConstructor(t *testing.T) {
	if n := runRule(t, `class A { static constructor() {} static 'constructor'() {} }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDupeClassMembers_NormalisesNumericKeys(t *testing.T) {
	if n := runRule(t, `class A { 10() {} 1e1() {} }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDupeClassMembers_DistinguishesNumericFromString(t *testing.T) {
	if n := runRule(t, `class A { [1.0]() {} ['1.0']() {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDupeClassMembers_IgnoresUnresolvedComputedIdentifierKeys(t *testing.T) {
	if n := runRule(t, `class A { [foo]() {} [foo]() {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDupeClassMembers_IgnoresTypeScriptOverloadSignatures(t *testing.T) {
	if n := runRule(t, `class A { foo(a: string): string; foo(a: number): number; foo(a: any): any { return a; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDupeClassMembers_AllowsSameNameAcrossDifferentClasses(t *testing.T) {
	if n := runRule(t, `class A { foo() {} } class B { foo() {} }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
