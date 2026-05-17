package norestrictedelements_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/norestrictedelements"
)

const tsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "jsx": "preserve"
  },
  "include": ["case.tsx"]
}`

func TestNoRestrictedElements_FlagsConfiguredTag(t *testing.T) {
	count := runRule(t, `const x = <img src="a.png" />;`,
		&norestrictedelements.Options{Elements: map[string]string{"img": "use <NextImage>"}})
	if count != 1 {
		t.Fatalf("want 1 diagnostic, got %d", count)
	}
}

func TestNoRestrictedElements_LeavesUnlistedTagAlone(t *testing.T) {
	count := runRule(t, `const x = <div />;`,
		&norestrictedelements.Options{Elements: map[string]string{"img": "use <NextImage>"}})
	if count != 0 {
		t.Fatalf("want 0 diagnostics, got %d", count)
	}
}

func TestNoRestrictedElements_IsOffWhenOptionsMissing(t *testing.T) {
	count := runRule(t, `const x = <img src="a.png" />;`, nil)
	if count != 0 {
		t.Fatalf("want 0 diagnostics when no options configured, got %d", count)
	}
}

func TestNoRestrictedElements_FlagsOpeningAndSelfClosingForms(t *testing.T) {
	count := runRule(t, `const a = <span />; const b = <span></span>;`,
		&norestrictedelements.Options{Elements: map[string]string{"span": ""}})
	if count != 2 {
		t.Fatalf("want 2 diagnostics across both JSX forms, got %d", count)
	}
}

func runRule(t *testing.T, code string, opts *norestrictedelements.Options) int {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-nre-")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(tsconfigBody), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "case.tsx"), []byte(code), 0o644); err != nil {
		t.Fatalf("write case: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{norestrictedelements.New()},
		map[string]wrapperlint.Severity{"no-restricted-elements": wrapperlint.SeverityError},
	)
	if opts != nil {
		eng = eng.WithOptions(map[string]any{"no-restricted-elements": opts})
	}
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-restricted-elements" {
			count++
		}
	}
	return count
}
