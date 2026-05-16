package nousebeforedefine_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nousebeforedefine"
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
		map[string]wrapperlint.Severity{"no-use-before-define": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoUseBeforeDefine_FlagsVarUsedBeforeDeclared(t *testing.T) {
	if n := runRule(t, `a++; var a=19;`, nousebeforedefine.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUseBeforeDefine_AllowsVarAfterDeclaration(t *testing.T) {
	if n := runRule(t, `var a=10; alert(a);`, nousebeforedefine.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUseBeforeDefine_FlagsFunctionUsedBeforeDeclaredByDefault(t *testing.T) {
	if n := runRule(t, `a(); function a() {}`, nousebeforedefine.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoUseBeforeDefine_AllowsFunctionUsedBeforeDeclaredWhenNofunc(t *testing.T) {
	opts := nousebeforedefine.DefaultOptions()
	opts.Functions = false
	if n := runRule(t, `a(); function a() {}`, nousebeforedefine.NewWithOptions(opts)); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoUseBeforeDefine_FlagsClassUsedBeforeDeclared(t *testing.T) {
	if n := runRule(t, `new A(); class A {}`, nousebeforedefine.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}
