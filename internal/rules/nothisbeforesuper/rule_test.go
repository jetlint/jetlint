package nothisbeforesuper_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nothisbeforesuper"
)

const tsconfigBody = `{"compilerOptions":{"strict":false,"target":"es2022","module":"esnext","moduleResolution":"bundler","allowJs":true,"noImplicitAny":false}}`

func runRule(t *testing.T, code string) int {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigBody), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(code), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nothisbeforesuper.New()},
		map[string]wrapperlint.Severity{"no-this-before-super": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoThisBeforeSuper_FlagsThisBeforeSuperInDerivedConstructor(t *testing.T) {
	if n := runRule(t, `class A extends B { constructor() { this.c = 0; super(); } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoThisBeforeSuper_AllowsThisAfterSuper(t *testing.T) {
	if n := runRule(t, `class A extends B { constructor() { super(); this.c = 0; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoThisBeforeSuper_AllowsThisInNonDerivedConstructor(t *testing.T) {
	if n := runRule(t, `class A { constructor() { this.c = 0; } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoThisBeforeSuper_FlagsThisPassedAsSuperArgument(t *testing.T) {
	if n := runRule(t, `class A extends B { constructor() { super(this); } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoThisBeforeSuper_AllowsThisAfterSuperInBothIfBranches(t *testing.T) {
	if n := runRule(t, `class A extends B { constructor() { if (a) super(); else super(); this.a(); } }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoThisBeforeSuper_FlagsThisAfterSuperInOnlyOneIfBranch(t *testing.T) {
	if n := runRule(t, `class A extends B { constructor() { if (a) super(); this.a(); } }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
