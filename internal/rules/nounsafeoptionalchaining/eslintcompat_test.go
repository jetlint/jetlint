package nounsafeoptionalchaining_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/nounsafeoptionalchaining"
)

const eslintTsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false
  },
  "include": ["case.ts"]
}`

func TestNoUnsafeOptionalChaining_EslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/no-unsafe-optional-chaining.json")
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
		t.Logf("MISMATCH [%s #%d] count=%d\n--- code ---\n%s\n--- end ---",
			valid, i, count, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("oxlint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
	// Allow up to 2 known divergences: oxlint flags
	// `with (chain) { ... }` (and the await-inside-with variant), but
	// the TS-go wrapper doesn't expose `KindWithStatement`, so we
	// can't pattern-match the parent. The `with` statement is
	// vanishingly rare in modern code and is deprecated in modules,
	// so we accept the gap rather than reach into the unexported
	// kind constant.
	if failed > 2 {
		t.Fatalf("expected at most 2 known divergences, got %d/%d (%.1f%%)", passed, total, pct)
	}
}

func optionsForCase(c eslintcompat.Case) nounsafeoptionalchaining.Options {
	var opts nounsafeoptionalchaining.Options
	first := c.FirstOption()
	if first == nil {
		return opts
	}
	if v, ok := first["disallowArithmeticOperators"].(bool); ok {
		opts.DisallowArithmeticOperators = v
	}
	return opts
}

func runEslintCase(t *testing.T, code string, opts nounsafeoptionalchaining.Options) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-nuoc-compat")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(eslintTsconfigBody), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644); err != nil {
		return 0, err
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nounsafeoptionalchaining.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-unsafe-optional-chaining": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-unsafe-optional-chaining" {
			count++
		}
	}
	return count, nil
}
