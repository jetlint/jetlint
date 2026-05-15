package nodupekeys_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nodupekeys"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-ndk")
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
		[]engine.Rule{nodupekeys.New()},
		map[string]wrapperlint.Severity{"no-dupe-keys": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoDupeKeys_FlagsDuplicateIdentifierKey(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { a: 1, a: 2 };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "no-dupe-keys" {
		t.Errorf("rule id = %q; want no-dupe-keys", diags[0].RuleID)
	}
	if !strings.Contains(diags[0].Message, "a") {
		t.Errorf("message should name the duplicated key; got %q", diags[0].Message)
	}
}

func TestNoDupeKeys_FlagsDuplicateStringKey(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { "a": 1, "a": 2 };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDupeKeys_FlagsDuplicateNumericKey(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { 1: "a", 1: "b" };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDupeKeys_FlagsMixedNumericAndStringKey(t *testing.T) {
	// JavaScript treats {1: ..., "1": ...} as duplicates because both
	// keys canonicalize to the string "1".
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { 1: "a", "1": "b" };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDupeKeys_FlagsShorthandWithLonghand(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const a = 1; const obj = { a, a: 2 };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDupeKeys_FlagsTwoGettersWithSameName(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { get a() { return 1; }, get a() { return 2; } };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestNoDupeKeys_DoesNotFlagGetterSetterPairForSameName(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `let _a = 0; const obj = { get a() { return _a; }, set a(v: number) { _a = v; } };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("get/set pair should not be flagged; got %d diagnostics: %#v", len(diags), diags)
	}
}

func TestNoDupeKeys_FlagsInitAfterGetterSetterPair(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `let _a = 0; const obj = { get a() { return _a; }, set a(v: number) { _a = v; }, a: 5 };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for the third 'a' (init); got %d", len(diags))
	}
}

func TestNoDupeKeys_DoesNotFlagUniqueKeys(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { a: 1, b: 2, c: 3, d: 4 };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestNoDupeKeys_TreatsNestedObjectLiteralsIndependently(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { a: 1, b: { a: 1, c: 2 } };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("nested 'a' should not collide with outer 'a'; got %d", len(diags))
	}
}

func TestNoDupeKeys_DoesNotFlagComputedKeys(t *testing.T) {
	// Computed keys may evaluate to anything at runtime; the rule
	// stays silent rather than guessing.
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const k = "a"; const obj = { [k]: 1, a: 2 };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Fatalf("computed key should not be flagged; got %d", len(diags))
	}
}

func TestNoDupeKeys_FlagsEachExtraOccurrence(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": `const obj = { a: 1, a: 2, a: 3 };
`,
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics (second and third 'a'); got %d", len(diags))
	}
}
