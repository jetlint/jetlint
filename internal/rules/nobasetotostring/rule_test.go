package nobasetotostring_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/nobasetotostring"
)

func fixture(t *testing.T, source string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`),
		0o644); err != nil {
		t.Fatalf("tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(source), 0o644); err != nil {
		t.Fatalf("write: %v", err)
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
		[]engine.Rule{nobasetotostring.New()},
		map[string]wrapperlint.Severity{"no-base-to-string": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoBaseToString_FlagsObjectInterpolation(t *testing.T) {
	tsconfig := fixture(t, "const user = { id: 1, name: \"ada\" };\nconst msg = `new user: ${user}`;\n")
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "no-base-to-string" {
		t.Errorf("expected rule id 'no-base-to-string', got %q", diags[0].RuleID)
	}
}

func TestNoBaseToString_DoesNotFlagPrimitiveInterpolation(t *testing.T) {
	tsconfig := fixture(t, "const name = \"ada\";\nconst age = 42;\nconst msg = `${name} is ${age}`;\n")
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for primitive interpolation, got %d: %#v", len(diags), diags)
	}
}

func TestNoBaseToString_DoesNotFlagObjectWithCustomToString(t *testing.T) {
	tsconfig := fixture(t, "class User {\n"+
		"  constructor(public name: string) {}\n"+
		"  toString(): string { return this.name; }\n"+
		"}\n"+
		"const u = new User(\"ada\");\n"+
		"const msg = `user: ${u}`;\n")
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for class with custom toString, got %d: %#v", len(diags), diags)
	}
}
