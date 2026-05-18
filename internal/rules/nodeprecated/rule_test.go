package nodeprecated_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nodeprecated"
)

// Regression for jetlint#625: probing the deprecation status of a
// `default`-property access on a CommonJS-shaped namespace import (e.g.
// `isbotModule.default(userAgent)` where isbotModule is the `* as`
// namespace import of a CJS module) panicked in the wrapper's
// IsDeprecated() walk because the upstream alias-resolution path hits
// an unexpected-nil assertion on that particular symbol shape. The
// rule should treat panicky introspection as "not deprecated".
func TestNoDeprecated_DoesNotPanicOnNamespaceImportDefaultMemberAccess(t *testing.T) {
	dir := t.TempDir()
	tsconfig := filepath.Join(dir, "tsconfig.json")
	files := map[string]string{
		"tsconfig.json":         `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"node","esModuleInterop":true,"allowSyntheticDefaultImports":true,"lib":["es2022","dom"]}}`,
		"node_modules/dep/package.json": `{"name":"dep","main":"index.js","types":"index.d.ts"}`,
		"node_modules/dep/index.d.ts":   "declare function isbot(ua?: string): boolean;\ndeclare namespace isbot {}\nexport = isbot;\n",
		"main.ts":               "import * as isbotModule from 'dep';\nfunction useBot(ua: string) {\n  if (typeof isbotModule === 'function') return isbotModule(ua);\n  if ('default' in isbotModule && typeof isbotModule.default === 'function') return isbotModule.default(ua);\n  return false;\n}\n",
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nodeprecated.New()},
		map[string]wrapperlint.Severity{"no-deprecated": wrapperlint.SeverityError},
	)
	diags := eng.Lint(prog)
	for _, d := range diags {
		if d.RuleID == "jetlint/rule-panic" {
			t.Fatalf("expected no rule-panic, got: %#v", diags)
		}
	}
}
