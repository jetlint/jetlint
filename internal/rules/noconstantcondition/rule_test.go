package noconstantcondition_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noconstantcondition"
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
		map[string]wrapperlint.Severity{"no-constant-condition": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoConstantCondition_FlagsLiteralTrueInIf(t *testing.T) {
	if n := runRule(t, `if (true) {}`, noconstantcondition.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstantCondition_FlagsLiteralNumber(t *testing.T) {
	if n := runRule(t, `if (1) {}`, noconstantcondition.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoConstantCondition_AllowsIdentifierTest(t *testing.T) {
	if n := runRule(t, `if (a) {}`, noconstantcondition.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoConstantCondition_AllowsWhileTrueByDefault(t *testing.T) {
	if n := runRule(t, `while (true) {}`, noconstantcondition.New()); n != 0 {
		t.Errorf("expected 0 diagnostics (default opts allowsExceptWhileTrue), got %d", n)
	}
}

func TestNoConstantCondition_FlagsWhileTrueWhenCheckAll(t *testing.T) {
	rule := noconstantcondition.NewWithOptions(noconstantcondition.Options{CheckLoops: noconstantcondition.CheckLoopsAll})
	if n := runRule(t, `while (true) {}`, rule); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
