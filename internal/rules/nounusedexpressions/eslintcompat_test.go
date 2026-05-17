package nounusedexpressions_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/nounusedexpressions"
)

const eslintTsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false,
    "jsx": "preserve"
  },
  "include": ["case.tsx"]
}`

func TestNoUnusedExpressions_EslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/no-unused-expressions.json")
	fx, err := eslintcompat.Load(fixturePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for i, c := range fx.Cases {
		opts := optionsForCase(c)
		count, runErr := runEslintCase(t, c.Code, opts)
		if runErr != nil {
			failed++
			t.Logf("FAIL [#%d] runCase: %v\n--- code ---\n%s\n--- end ---", i, runErr, c.Code)
			continue
		}
		ok := (c.Valid && count == 0) || (!c.Valid && count >= 1)
		if ok {
			passed++
			continue
		}
		failed++
		valid := "fail"
		if c.Valid {
			valid = "pass"
		}
		t.Logf("MISMATCH [%s #%d] count=%d opts=%+v\n--- code ---\n%s\n--- end ---",
			valid, i, count, opts, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("oxlint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
	if failed > 0 {
		t.Fatalf("expected 100%% pass rate, got %d/%d (%.1f%%)", passed, total, pct)
	}
}

func optionsForCase(c eslintcompat.Case) nounusedexpressions.Options {
	opts := nounusedexpressions.DefaultOptions()
	first := c.FirstOption()
	if first == nil {
		return opts
	}
	if v, ok := first["allowShortCircuit"].(bool); ok {
		opts.AllowShortCircuit = v
	}
	if v, ok := first["allowTernary"].(bool); ok {
		opts.AllowTernary = v
	}
	if v, ok := first["allowTaggedTemplates"].(bool); ok {
		opts.AllowTaggedTemplates = v
	}
	if v, ok := first["enforceForJSX"].(bool); ok {
		opts.EnforceForJSX = v
	}
	return opts
}

func runEslintCase(t *testing.T, code string, opts nounusedexpressions.Options) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-nue-compat")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(eslintTsconfigBody), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "case.tsx"), []byte(code), 0o644); err != nil {
		return 0, err
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nounusedexpressions.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-unused-expressions": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-unused-expressions" {
			count++
		}
	}
	return count, nil
}
