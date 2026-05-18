package nouselessbackreference_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nouselessbackreference"
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
		[]engine.Rule{nouselessbackreference.New()},
		map[string]wrapperlint.Severity{"no-useless-backreference": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUselessBackreference_FlagsRefToMissingNumberedGroup(t *testing.T) {
	if n := runRule(t, `var r = /(a)\2/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUselessBackreference_AllowsRefToExistingGroup(t *testing.T) {
	if n := runRule(t, `var r = /(a)\1/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUselessBackreference_FlagsRefToMissingNamedGroup(t *testing.T) {
	if n := runRule(t, `var r = /(?<foo>a)\k<bar>/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUselessBackreference_AllowsRefToExistingNamedGroup(t *testing.T) {
	if n := runRule(t, `var r = /(?<foo>a)\k<foo>/;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUselessBackreference_DoesNotCountNonCapturingGroups(t *testing.T) {
	if n := runRule(t, `var r = /(?:a)\1/;`); n != 1 {
		t.Errorf("expected 1 diagnostic for unbacked \\1, got %d", n)
	}
}

func TestNoUselessBackreference_DoesNotCountLookahead(t *testing.T) {
	if n := runRule(t, `var r = /(?=a)\1/;`); n != 1 {
		t.Errorf("expected 1 diagnostic for unbacked \\1, got %d", n)
	}
}

func TestNoUselessBackreference_IgnoresEscapeInsideCharClass(t *testing.T) {
	if n := runRule(t, `var r = /(a)[\1]/;`); n != 0 {
		t.Errorf("expected 0 diagnostics (\\1 inside class is literal), got %d", n)
	}
}

func TestNoUselessBackreference_FlagsMultipleGroups(t *testing.T) {
	if n := runRule(t, `var r = /(a)(b)\5/;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUselessBackreference_AllowsMultiDigitGroupCount(t *testing.T) {
	if n := runRule(t, `var r = /(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\10/;`); n != 0 {
		t.Errorf("expected 0 diagnostics (\\10 refers to 10th group), got %d", n)
	}
}

func TestNoUselessBackreference_FlagsCircularNumberedRef(t *testing.T) {
	if n := runRule(t, `var r = /(a\1)/;`); n != 1 {
		t.Errorf("expected 1 diagnostic for circular \\1 inside group 1, got %d", n)
	}
}

func TestNoUselessBackreference_FlagsCircularNamedRef(t *testing.T) {
	if n := runRule(t, `var r = /(?<foo>a\k<foo>)/;`); n != 1 {
		t.Errorf("expected 1 diagnostic for circular \\k<foo> inside (?<foo>...), got %d", n)
	}
}

func TestNoUselessBackreference_AllowsForwardRefOutsideContainingGroup(t *testing.T) {
	if n := runRule(t, `var r = /((a)\2)/;`); n != 0 {
		t.Errorf("expected 0 diagnostics for \\2 after group 2 closes, got %d", n)
	}
}
