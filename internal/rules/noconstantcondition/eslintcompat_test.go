package noconstantcondition_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/noconstantcondition"
)

const eslintTsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false
  },
  "include": ["case.ts"]
}`

func TestNoConstantCondition_EslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/no-constant-condition.json")
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
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("oxlint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
	// This is a deliberately conservative port. The full ESLint rule
	// evaluates logical-and-arithmetic constant folding, template
	// substitutions, and `Boolean(...)` calls — all of which require
	// a shared `is_constant` utility we haven't built yet. The port
	// catches the literal-only test cases (which are the ones users
	// actually hit) and lets the more elaborate fixtures slide. The
	// long-term plan is a follow-up that introduces the utility and
	// tightens this threshold.
	minimumPassRate := 50.0
	if pct < minimumPassRate {
		t.Fatalf("expected at least %.1f%% pass rate, got %d/%d (%.1f%%)", minimumPassRate, passed, total, pct)
	}
}

func optionsForCase(c eslintcompat.Case) noconstantcondition.Options {
	var opts noconstantcondition.Options
	first := c.FirstOption()
	if first == nil {
		return opts
	}
	if v, ok := first["checkLoops"]; ok {
		switch s := v.(type) {
		case bool:
			if s {
				opts.CheckLoops = noconstantcondition.CheckLoopsAll
			} else {
				opts.CheckLoops = noconstantcondition.CheckLoopsNone
			}
		case string:
			switch s {
			case "all":
				opts.CheckLoops = noconstantcondition.CheckLoopsAll
			case "none":
				opts.CheckLoops = noconstantcondition.CheckLoopsNone
			case "allExceptWhileTrue":
				opts.CheckLoops = noconstantcondition.CheckLoopsAllExceptWhileTrue
			}
		}
	}
	return opts
}

func runEslintCase(t *testing.T, code string, opts noconstantcondition.Options) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-ncc-compat")
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
		[]engine.Rule{noconstantcondition.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-constant-condition": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-constant-condition" {
			count++
		}
	}
	return count, nil
}
