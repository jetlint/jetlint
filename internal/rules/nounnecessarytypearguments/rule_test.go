package nounnecessarytypearguments_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypearguments"
)

func runOn(t *testing.T, files map[string]string) []wrapperlint.Diagnostic {
	t.Helper()
	dir := t.TempDir()
	if _, has := files["tsconfig.json"]; !has {
		files["tsconfig.json"] = `{"compilerOptions":{"jsx":"react-jsx","strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler","lib":["es2022","dom"]}}`
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	prog, err := wrapperchecker.LoadProgram(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	eng := engine.New(
		[]engine.Rule{nounnecessarytypearguments.New()},
		map[string]wrapperlint.Severity{"no-unnecessary-type-arguments": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

// Regression for jetlint#625: comparing the user-supplied type
// arguments to the declared defaults invoked the wrapper's
// TypeArguments() unsafely; on type references whose target lacks
// InterfaceType backing (`React.FC<P>` style declarations from
// .d.ts), the wrapper panicked.
func TestNoUnnecessaryTypeArguments_DoesNotPanicOnReactFCStyleTypeReference(t *testing.T) {
	diags := runOn(t, map[string]string{
		"shim.d.ts": "declare namespace React { type FC<P = {}> = (props: P) => unknown; }\n",
		"main.tsx":  "interface Props { size?: string }\nexport const C: React.FC<Props> = () => null as any;\n",
	})
	for _, d := range diags {
		if d.RuleID == "jetlint/rule-panic" {
			t.Fatalf("expected no rule-panic, got: %#v", diags)
		}
	}
}
