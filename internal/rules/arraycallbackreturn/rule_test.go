package arraycallbackreturn_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/arraycallbackreturn"
)

func fixture(t *testing.T, code string) string {
	t.Helper()
	dir := t.TempDir()
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(code), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	return tsc
}

func runRule(t *testing.T, code string, opts arraycallbackreturn.Options) []wrapperlint.Diagnostic {
	t.Helper()
	prog, err := wrapperchecker.LoadProgram(fixture(t, code))
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{arraycallbackreturn.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"array-callback-return": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestArrayCallbackReturn_FlagsMapCallbackThatNeverReturns(t *testing.T) {
	diags := runRule(t, "[1,2].map(function(x) { console.log(x); });", arraycallbackreturn.Options{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	if diags[0].RuleID != "array-callback-return" {
		t.Errorf("rule id = %q", diags[0].RuleID)
	}
	if !strings.Contains(diags[0].Message, "map") {
		t.Errorf("message should name the array method; got %q", diags[0].Message)
	}
}

func TestArrayCallbackReturn_DoesNotFlagMapCallbackThatReturns(t *testing.T) {
	diags := runRule(t, "[1,2].map(function(x) { return x; });", arraycallbackreturn.Options{})
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestArrayCallbackReturn_DoesNotFlagConciseBodyArrow(t *testing.T) {
	diags := runRule(t, "[1,2].map(x => x);", arraycallbackreturn.Options{})
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestArrayCallbackReturn_FlagsFilterCallbackWithMissingElseBranch(t *testing.T) {
	diags := runRule(t, "[1,2].filter(function(x) { if (x) return true; });", arraycallbackreturn.Options{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestArrayCallbackReturn_AllowsBareReturnUnderAllowImplicit(t *testing.T) {
	diags := runRule(t, "[1,2].filter(function(x) { if (x) return true; return; });",
		arraycallbackreturn.Options{AllowImplicit: true})
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestArrayCallbackReturn_RejectsBareReturnByDefault(t *testing.T) {
	diags := runRule(t, "[1,2].filter(function(x) { if (x) return true; return; });",
		arraycallbackreturn.Options{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic without allowImplicit, got %d", len(diags))
	}
}

func TestArrayCallbackReturn_IgnoresForEachByDefault(t *testing.T) {
	diags := runRule(t, "[1,2].forEach(function(x) { return x; });", arraycallbackreturn.Options{})
	if len(diags) != 0 {
		t.Fatalf("forEach should not be checked by default; got %d", len(diags))
	}
}

func TestArrayCallbackReturn_CheckForEachFlagsExplicitReturn(t *testing.T) {
	diags := runRule(t, "[1,2].forEach(function(x) { return x; });",
		arraycallbackreturn.Options{CheckForEach: true})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic under checkForEach, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "forEach") {
		t.Errorf("message should name forEach; got %q", diags[0].Message)
	}
}

func TestArrayCallbackReturn_FlagsAllThirteenArrayMethods(t *testing.T) {
	methods := []string{"every", "filter", "find", "findIndex", "findLast", "findLastIndex",
		"flatMap", "map", "reduce", "reduceRight", "some", "sort", "toSorted"}
	for _, m := range methods {
		code := "([] as any)." + m + "(function(x) { console.log(x); });"
		diags := runRule(t, code, arraycallbackreturn.Options{})
		if len(diags) == 0 {
			t.Errorf("%s: expected at least 1 diagnostic; got 0", m)
		}
	}
}

func TestArrayCallbackReturn_DoesNotFlagUnrelatedCalls(t *testing.T) {
	diags := runRule(t, "foo(function(x) { console.log(x); });", arraycallbackreturn.Options{})
	if len(diags) != 0 {
		t.Fatalf("plain function call should not be checked; got %d", len(diags))
	}
}

func TestArrayCallbackReturn_DoesNotFlagWhenCallbackIsIdentifier(t *testing.T) {
	diags := runRule(t, "function cb(x: number): number { return x; }\n[1,2].map(cb);",
		arraycallbackreturn.Options{})
	if len(diags) != 0 {
		t.Fatalf("identifier callback is out of scope; got %d", len(diags))
	}
}

func TestArrayCallbackReturn_FlagsBracketAccessMap(t *testing.T) {
	diags := runRule(t, "[1,2]['map'](function(x) { console.log(x); });", arraycallbackreturn.Options{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for bracket access; got %d", len(diags))
	}
}
