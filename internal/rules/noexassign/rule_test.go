package noexassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noexassign"
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
		[]engine.Rule{noexassign.New()},
		map[string]wrapperlint.Severity{"no-ex-assign": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoExAssign_FlagsDirectAssignment(t *testing.T) {
	if n := runRule(t, `try {} catch (e) { e = 10; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoExAssign_AllowsAssignmentToOtherVariable(t *testing.T) {
	if n := runRule(t, `try {} catch (e) { let other = 10; other = 20; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoExAssign_FlagsAssignmentToDestructuredBinding(t *testing.T) {
	if n := runRule(t, `try {} catch ({message}) { message = 10; }`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
