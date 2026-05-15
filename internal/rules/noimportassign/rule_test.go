package noimportassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noimportassign"
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
		[]engine.Rule{noimportassign.New()},
		map[string]wrapperlint.Severity{"no-import-assign": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoImportAssign_FlagsDirectAssign(t *testing.T) {
	if n := runRule(t, `import mod from 'mod'; mod = 0;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoImportAssign_FlagsNamedAssign(t *testing.T) {
	if n := runRule(t, `import {named} from 'mod'; named = 0;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoImportAssign_FlagsNamespaceMemberAssign(t *testing.T) {
	if n := runRule(t, `import * as mod from 'mod'; mod.x = 0;`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoImportAssign_AllowsDefaultMemberAssign(t *testing.T) {
	if n := runRule(t, `import mod from 'mod'; mod.x = 0;`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoImportAssign_FlagsObjectAssignOnNamespace(t *testing.T) {
	if n := runRule(t, `import * as mod from 'mod'; Object.assign(mod, {});`); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoImportAssign_AllowsLocalShadow(t *testing.T) {
	if n := runRule(t, `import mod from 'mod'; { let mod = 0; mod = 1; }`); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
