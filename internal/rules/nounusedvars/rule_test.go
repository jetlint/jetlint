package nounusedvars_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounusedvars"
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
		[]engine.Rule{nounusedvars.New()},
		map[string]wrapperlint.Severity{"no-unused-vars": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUnusedVars_FlagsUnusedVariable(t *testing.T) {
	if n := runRule(t, `var x = 1;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedVars_FlagsUnusedFunction(t *testing.T) {
	if n := runRule(t, `function f() {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedVars_FlagsUnusedClass(t *testing.T) {
	if n := runRule(t, `class C {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedVars_AllowsReferencedVariable(t *testing.T) {
	if n := runRule(t, `var x = 1; console.log(x);`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedVars_AllowsExportedDeclaration(t *testing.T) {
	if n := runRule(t, `export function f() {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedVars_AllowsExportedConst(t *testing.T) {
	if n := runRule(t, `export const x = 1;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedVars_AllowsUnderscorePrefix(t *testing.T) {
	if n := runRule(t, `var _unused = 1;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedVars_FlagsUnusedImport(t *testing.T) {
	if n := runRule(t, `import { foo } from "./mod"; export {};`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnusedVars_AllowsUsedImport(t *testing.T) {
	if n := runRule(t, `import { foo } from "./mod"; foo();`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedVars_AllowsExportedImport(t *testing.T) {
	if n := runRule(t, `import { foo } from "./mod"; export { foo };`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnusedVars_FlagsMultipleInOneStatement(t *testing.T) {
	if n := runRule(t, `var a = 1, b = 2; console.log(a);`); n != 1 {
		t.Errorf("expected 1 diagnostic for b, got %d", n)
	}
}

func TestNoUnusedVars_AllowsDestructuringRead(t *testing.T) {
	if n := runRule(t, `const { x } = obj; console.log(x);`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
