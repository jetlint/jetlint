package useexhaustivedependencies_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/useexhaustivedependencies"
)

func TestDebug38(t *testing.T) {
	code, err := os.ReadFile("/tmp/uhtl-35.tsx")
	if err != nil {
		t.Skip("no debug file")
	}
	dir, err := os.MkdirTemp("/tmp", "jl-ued-debug")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(eslintTsconfigBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "case.tsx"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		t.Fatal(err)
	}
	defer prog.Close()
	_ = strings.Split // keep import alive
	eng := engine.New(
		[]engine.Rule{useexhaustivedependencies.New()},
		map[string]wrapperlint.Severity{"use-exhaustive-dependencies": wrapperlint.SeverityError},
	)
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "use-exhaustive-dependencies" {
			t.Logf("DIAG at %d:%d: %s", d.Range.StartLine, d.Range.StartColumn, d.Message)
		}
	}
}
