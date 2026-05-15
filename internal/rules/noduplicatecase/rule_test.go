package noduplicatecase_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noduplicatecase"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-ndc")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if _, hasConfig := files["tsconfig.json"]; !hasConfig {
		files["tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
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
		[]engine.Rule{noduplicatecase.New()},
		map[string]wrapperlint.Severity{"no-duplicate-case": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoDuplicateCase_FlagsDuplicateLiteralCases(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const x: number = 1;
switch (x) {
  case 1: break;
  case 2: break;
  case 1: break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "no-duplicate-case" {
		t.Errorf("rule id = %q; want no-duplicate-case", diags[0].RuleID)
	}
	if !strings.Contains(diags[0].Message, "duplicate") {
		t.Errorf("message should mention duplicate; got %q", diags[0].Message)
	}
}

func TestNoDuplicateCase_FlagsDuplicateStringCases(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const x: string = "a";
switch (x) {
  case "a": break;
  case "b": break;
  case "a": break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDuplicateCase_FlagsDuplicateIdentifierCases(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const FOO = 1;
const x: number = 1;
switch (x) {
  case FOO: break;
  case 2: break;
  case FOO: break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDuplicateCase_FlagsDuplicateMemberAccessCases(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const obj = { kind: 1 };
const x: number = 1;
switch (x) {
  case obj.kind: break;
  case 5: break;
  case obj.kind: break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDuplicateCase_FlagsEachDuplicateInLargeSwitch(t *testing.T) {
	// Three "1"s and two "2"s should produce diagnostics for every
	// repeat past the first: the second and third "1" and the second
	// "2". That's three diagnostics.
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const x: number = 1;
switch (x) {
  case 1: break;
  case 2: break;
  case 1: break;
  case 2: break;
  case 1: break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestNoDuplicateCase_DoesNotFlagUniqueCases(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const x: number = 1;
switch (x) {
  case 1: break;
  case 2: break;
  case 3: break;
  default: break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestNoDuplicateCase_TreatsSeparateSwitchesIndependently(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const x: number = 1;
switch (x) {
  case 1: break;
  case 2: break;
}
switch (x) {
  case 1: break;
  case 2: break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("identical cases in separate switches should not be flagged; got %d diagnostics", len(diags))
	}
}

func TestNoDuplicateCase_TreatsNestedSwitchesIndependently(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `
const x: number = 1;
switch (x) {
  case 1:
    switch (x) {
      case 1: break;
    }
    break;
  case 2: break;
}
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("inner case 1 should not collide with outer case 1; got %d", len(diags))
	}
}
