package nodupeelseif_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nodupeelseif"
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
		[]engine.Rule{nodupeelseif.New()},
		map[string]wrapperlint.Severity{"no-dupe-else-if": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoDupeElseIf_FlagsDirectDuplicate(t *testing.T) {
	if n := runRule(t, `if (a) {} else if (a) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDupeElseIf_AllowsDistinctConditions(t *testing.T) {
	if n := runRule(t, `if (a) {} else if (b) {} else if (c) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDupeElseIf_TreatsParenthesizedDuplicateAsSame(t *testing.T) {
	if n := runRule(t, `if (a === 1) {} else if ((a === 1)) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDupeElseIf_FlagsOrSubsumedByEarlierOperand(t *testing.T) {
	if n := runRule(t, `if (a || b) {} else if (a) {}`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDupeElseIf_AllowsCommutativeOrWhenDistinct(t *testing.T) {
	if n := runRule(t, `if (a || b) {} else if (c || d) {}`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
