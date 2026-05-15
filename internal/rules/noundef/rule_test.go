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

const tsconfigBody = `{"compilerOptions":{"strict":false,"target":"es2022","module":"esnext","moduleResolution":"bundler","lib":["es2022","dom"],"allowJs":true,"noImplicitAny":false,"skipLibCheck":true}}`

func runRule(t *testing.T, code string, rule engine.Rule) int {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigBody), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(code), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{rule},
		map[string]wrapperlint.Severity{"no-undef": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUndef_AllowsDeclaredVar(t *testing.T) {
	if n := runRule(t, `var a = 1; a;`, noundef.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUndef_FlagsUndeclaredAssignment(t *testing.T) {
	if n := runRule(t, `a = 1;`, noundef.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUndef_AllowsTypeofUndeclared(t *testing.T) {
	if n := runRule(t, `typeof a;`, noundef.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUndef_AllowsGlobalsFromLib(t *testing.T) {
	if n := runRule(t, `Object; isNaN();`, noundef.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
