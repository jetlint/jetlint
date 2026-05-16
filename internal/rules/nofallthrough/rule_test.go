package nofallthrough_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nofallthrough"
)

const tsconfigBody = `{"compilerOptions":{"strict":false,"target":"es2022","module":"esnext","moduleResolution":"bundler","allowJs":true,"noImplicitAny":false}}`

func runRule(t *testing.T, code string, rule engine.Rule) int {
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
		[]engine.Rule{rule},
		map[string]wrapperlint.Severity{"no-fallthrough": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoFallthrough_FlagsCaseFallingIntoNext(t *testing.T) {
	if n := runRule(t, `switch(x) { case 0: a(); case 1: b(); }`, nofallthrough.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoFallthrough_AllowsCaseEndedWithBreak(t *testing.T) {
	if n := runRule(t, `switch(x) { case 0: a(); break; case 1: b(); }`, nofallthrough.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoFallthrough_AllowsCaseEndedWithReturn(t *testing.T) {
	if n := runRule(t, `function f() { switch(x) { case 0: return; case 1: b(); } }`, nofallthrough.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoFallthrough_AllowsFallthroughCommentBeforeNextCase(t *testing.T) {
	if n := runRule(t, `switch(x) { case 0: a(); /* falls through */ case 1: b(); }`, nofallthrough.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoFallthrough_AllowsEmptyCaseWithoutBlankLine(t *testing.T) {
	if n := runRule(t, `switch(x) { case 0: case 1: a(); break; }`, nofallthrough.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoFallthrough_FlagsEmptyCaseWithBlankLineByDefault(t *testing.T) {
	if n := runRule(t, "switch(x) { case 0:\n\ncase 1: b(); }", nofallthrough.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
