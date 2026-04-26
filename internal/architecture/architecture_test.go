// Package architecture_test enforces import-boundary invariants of the
// linter codebase. It scans Go source files at test time, parses their
// import declarations, and fails the build when a rule package or the
// linter as a whole reaches across an architectural boundary it should
// not. These tests are the executable form of the wrapper-API decision
// in the project plan.
package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const forkInternalPrefix = "github.com/microsoft/typescript-go/internal/"

// allowlistForRulePackages lists import-path prefixes that rule packages may
// depend on, in addition to the standard library. Anything else in a rule
// package fails the build.
var allowlistForRulePackages = []string{
	"github.com/tommymorgan/tsgolint/",
	"github.com/microsoft/typescript-go/pkg/",
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" && root != "/dev/null" {
			return filepath.Dir(root)
		}
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// importsOfFile returns the import paths declared in a Go source file.
func importsOfFile(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		out = append(out, strings.Trim(imp.Path.Value, "\""))
	}
	return out
}

// walkGoFiles invokes visit with every .go file under root, skipping vendor,
// node_modules, bin, and dot-prefixed directories.
func walkGoFiles(t *testing.T, root string, visit func(path string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		visit(path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func isStandardLibraryImport(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return !strings.Contains(first, ".")
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func TestArchitecture_LinterDoesNotImportForkInternals(t *testing.T) {
	root := repoRoot(t)
	walkGoFiles(t, root, func(path string) {
		for _, imp := range importsOfFile(t, path) {
			if strings.HasPrefix(imp, forkInternalPrefix) {
				t.Errorf("%s imports %s — internal packages of the typescript-go fork are off-limits; use the wrapper at github.com/microsoft/typescript-go/pkg/* instead", path, imp)
			}
		}
	})
}

func TestArchitecture_RulePackagesUseOnlyAllowlistedImports(t *testing.T) {
	rulesRoot := filepath.Join(repoRoot(t), "internal", "rules")
	info, err := os.Stat(rulesRoot)
	if err != nil || !info.IsDir() {
		// No rules exist yet; the architecture test is in place to start
		// enforcing the moment they do.
		t.Skip("no rule packages present to validate")
	}
	walkGoFiles(t, rulesRoot, func(path string) {
		for _, imp := range importsOfFile(t, path) {
			if isStandardLibraryImport(imp) {
				continue
			}
			if !hasAnyPrefix(imp, allowlistForRulePackages) {
				t.Errorf("%s imports %s — rule packages may only depend on the standard library, this module, or github.com/microsoft/typescript-go/pkg/*", path, imp)
			}
		}
	})
}
