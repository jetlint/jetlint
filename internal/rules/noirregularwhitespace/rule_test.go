package noirregularwhitespace_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noirregularwhitespace"
)

const tsconfigBody = `{"compilerOptions":{"strict":false,"target":"es2022","module":"esnext","moduleResolution":"bundler","allowJs":true,"noImplicitAny":false}}`

func runRule(t *testing.T, code string, rule engine.Rule) int {
	t.Helper()
	dir := t.TempDir()
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(tsconfigBody), 0o644); err != nil {
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
		map[string]wrapperlint.Severity{"no-irregular-whitespace": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoIrregularWhitespace_FlagsNonBreakingSpaceInCode(t *testing.T) {
	rule := noirregularwhitespace.New()
	// Non-breaking space between let and x.
	code := "let x = 1;"
	if n := runRule(t, code, rule); n == 0 {
		t.Errorf("expected diagnostic for NBSP in code, got 0")
	}
}

func TestNoIrregularWhitespace_AllowsNonBreakingSpaceInsideStringByDefault(t *testing.T) {
	rule := noirregularwhitespace.New()
	code := "const s = \"a b\";"
	if n := runRule(t, code, rule); n != 0 {
		t.Errorf("expected 0 diagnostics (skipStrings default), got %d", n)
	}
}

func TestNoIrregularWhitespace_FlagsNonBreakingSpaceInsideStringWhenSkipStringsOff(t *testing.T) {
	rule := noirregularwhitespace.NewWithOptions(noirregularwhitespace.Options{
		SkipStrings:   false,
		SkipComments:  true,
		SkipRegExps:   true,
		SkipTemplates: true,
	})
	code := "const s = \"a b\";"
	if n := runRule(t, code, rule); n == 0 {
		t.Errorf("expected diagnostic when skipStrings is false, got 0")
	}
}

func TestNoIrregularWhitespace_AllowsLeadingBomAtStartOfFile(t *testing.T) {
	rule := noirregularwhitespace.New()
	code := "\uFEFFlet x = 1;"
	if n := runRule(t, code, rule); n != 0 {
		t.Errorf("expected 0 diagnostics for leading BOM, got %d", n)
	}
}

func TestNoIrregularWhitespace_AllowsIrregularWhitespaceInsideLineComment(t *testing.T) {
	rule := noirregularwhitespace.New()
	code := "// hi there\nlet x = 1;"
	if n := runRule(t, code, rule); n != 0 {
		t.Errorf("expected 0 diagnostics (skipComments default), got %d", n)
	}
}

func TestNoIrregularWhitespace_FlagsIrregularWhitespaceInsideLineCommentWhenSkipCommentsOff(t *testing.T) {
	rule := noirregularwhitespace.NewWithOptions(noirregularwhitespace.Options{
		SkipStrings:   true,
		SkipComments:  false,
		SkipRegExps:   true,
		SkipTemplates: true,
	})
	code := "// hi there\nlet x = 1;"
	if n := runRule(t, code, rule); n == 0 {
		t.Errorf("expected diagnostic when skipComments is false, got 0")
	}
}
