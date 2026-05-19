package nounsafereturn_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounsafereturn"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if _, has := files["tsconfig.json"]; !has {
		files["tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler","lib":["es2022","dom"]}}`
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return filepath.Join(dir, "tsconfig.json")
}

func runRule(t *testing.T, tsconfig string) []wrapperlint.Diagnostic {
	t.Helper()
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nounsafereturn.New()},
		map[string]wrapperlint.Severity{"no-unsafe-return": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

// Regression for jetlint#625: returning a value with a complex type
// (DOM-builtin, recursive interface, generic instantiation) from a
// function with no declared return type panicked because the rule's
// `typeContainsAnyDeep` walk called the wrapper's `TypeArguments()`,
// which propagates a panic on TypeReferences whose target lacks
// InterfaceType backing.
func TestNoUnsafeReturn_DoesNotPanicReturningDateFromInferredReturnTypeArrow(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "declare const selected: Date | null;\nconst fn = () => {\n  if (selected) return selected;\n  return new Date();\n};\n",
	})
	diags := runRule(t, tsconfig)
	for _, d := range diags {
		if d.RuleID == "jetlint/rule-panic" {
			t.Fatalf("expected no rule-panic, got: %#v", diags)
		}
	}
}
