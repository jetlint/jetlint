package noduplicateimports_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noduplicateimports"
)

func runRule(t *testing.T, code string, rule engine.Rule) int {
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
		[]engine.Rule{rule},
		map[string]wrapperlint.Severity{"no-duplicate-imports": wrapperlint.SeverityError},
	)
	return len(eng.Lint(prog))
}

func TestNoDuplicateImports_FlagsTwoNamedImports(t *testing.T) {
	if n := runRule(t, `import { merge } from "lodash-es"; import { find } from "lodash-es";`, noduplicateimports.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDuplicateImports_AllowsNamespacePlusNamed(t *testing.T) {
	if n := runRule(t, `import * as bar from "os"; import { baz } from "os";`, noduplicateimports.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}

func TestNoDuplicateImports_FlagsDefaultPlusNamed(t *testing.T) {
	if n := runRule(t, `import { merge } from "lodash-es"; import _ from "lodash-es";`, noduplicateimports.New()); n != 1 {
		t.Errorf("expected 1 diagnostic, got %d", n)
	}
}

func TestNoDuplicateImports_AllowsDifferentSources(t *testing.T) {
	if n := runRule(t, `import os from "os"; import fs from "fs";`, noduplicateimports.New()); n != 0 {
		t.Errorf("expected 0 diagnostics, got %d", n)
	}
}
