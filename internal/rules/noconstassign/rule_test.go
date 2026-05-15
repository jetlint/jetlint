package noconstassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noconstassign"
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
		[]engine.Rule{noconstassign.New()},
		map[string]wrapperlint.Severity{"no-const-assign": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoConstAssign_FlagsDirectReassignmentOfConst(t *testing.T) {
	if n := runRule(t, `const a = 0; a = 1;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstAssign_FlagsCompoundAssignmentOfConst(t *testing.T) {
	if n := runRule(t, `const a = 0; a += 1;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstAssign_FlagsIncrementOfConst(t *testing.T) {
	if n := runRule(t, `const a = 0; a++;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstAssign_FlagsConstBoundViaDestructure(t *testing.T) {
	if n := runRule(t, `const {a: x} = {a: 0}; x = 1;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstAssign_FlagsConstBoundViaArrayRest(t *testing.T) {
	if n := runRule(t, `const [a, ...b] = [1,2,3]; b = [];`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstAssign_AllowsPropertyAssignmentOnConstObject(t *testing.T) {
	if n := runRule(t, `const a = {k: 0}; a.k = 1;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstAssign_AllowsAssignmentOfLet(t *testing.T) {
	if n := runRule(t, `let a = 0; a = 1;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstAssign_AllowsParameterShadowingConst(t *testing.T) {
	if n := runRule(t, `const x = 0; function f(x: any) { x = 1; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstAssign_AllowsForOfBinding(t *testing.T) {
	if n := runRule(t, `for (const x of [1,2,3]) { foo(x); }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
