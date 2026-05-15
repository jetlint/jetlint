package nounsafenegation_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounsafenegation"
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
		map[string]wrapperlint.Severity{"no-unsafe-negation": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUnsafeNegation_FlagsBangInIn(t *testing.T) {
	if n := runRule(t, `if (!a in b) {}`, nounsafenegation.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnsafeNegation_FlagsBangInInstanceOf(t *testing.T) {
	if n := runRule(t, `if (!a instanceof b) {}`, nounsafenegation.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUnsafeNegation_AllowsParenthesizedBang(t *testing.T) {
	if n := runRule(t, `if ((!a) in b) {}`, nounsafenegation.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnsafeNegation_AllowsNegationOfInResult(t *testing.T) {
	if n := runRule(t, `if (!(a in b)) {}`, nounsafenegation.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnsafeNegation_DefaultOptionIgnoresOrderingRelations(t *testing.T) {
	if n := runRule(t, `if (!a < b) {}`, nounsafenegation.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUnsafeNegation_EnforceForOrderingRelationsFlagsLessThan(t *testing.T) {
	rule := nounsafenegation.NewWithOptions(nounsafenegation.Options{EnforceForOrderingRelations: true})
	if n := runRule(t, `if (!a < b) {}`, rule); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
