package noundef_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noundef"
)

// Regression for jetlint#625: scanning a containing scope for a
// sibling binding called LiteralText() on each variable declaration's
// name. For destructuring declarations like
// `const { threads, count: threadCount } = ...` the name node is a
// BindingPattern (not an identifier), and LiteralText() panics on that
// node shape. The rule should ignore non-identifier declaration names
// since they contribute their bound identifiers through inner
// BindingElement nodes that this scope walk visits separately.
func TestNoUndef_DoesNotPanicOnDestructuringDeclarationInSurroundingScope(t *testing.T) {
	dir := t.TempDir()
	tsconfig := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsconfig, []byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler","lib":["es2022"]}}`), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	code := "declare function fetchPair(): { threads: string[]; count: number };\ndeclare const obj: { a: string; b: string };\nfunction load() {\n  const { a, b } = obj;\n  let threads: string[] = [];\n  let threadCount = 0;\n  ({ threads, count: threadCount } = fetchPair());\n  return { a, b, threads, threadCount };\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(code), 0o644); err != nil {
		t.Fatalf("write case: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{noundef.New()},
		map[string]wrapperlint.Severity{"no-undef": wrapperlint.SeverityError},
	)
	diags := eng.Lint(prog)
	for _, d := range diags {
		if d.RuleID == "jetlint/rule-panic" {
			t.Fatalf("expected no rule-panic, got: %#v", diags)
		}
	}
}
