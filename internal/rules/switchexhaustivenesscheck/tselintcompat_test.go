package switchexhaustivenesscheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/switchexhaustivenesscheck"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

// fixtureTsconfigTemplate renders the per-case tsconfig. Cases whose
// `languageOptions.parserOptions.project` references the
// `tsconfig.noUncheckedIndexedAccess.json` fixture toggle on
// `noUncheckedIndexedAccess` so `x[0]` types include `undefined` —
// upstream fixtures that depend on that narrowing assume this option.
func fixtureTsconfigTemplate(noUnchecked bool) string {
	extra := ""
	if noUnchecked {
		extra = `, "noUncheckedIndexedAccess": true`
	}
	return `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true` + extra + `
  },
  "include": ["case.ts", "switch-exhaustiveness-check.ts"]
}`
}

// fixtureNamespacedEnumModule mirrors upstream's
// `tests/fixtures/switch-exhaustiveness-check.ts` so cases that
// `import { A } from './switch-exhaustiveness-check'` and switch on
// `A.B` can resolve `A.B.C` / `A.B.D` to enum-member literals — the
// rule then has the union members it needs for missing-branch detection.
const fixtureNamespacedEnumModule = `export namespace A {
  export enum B {
    C,
    D,
  }
}
`

func TestSwitchExhaustivenessCheck_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/switch-exhaustiveness-check.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "switch-exhaustiveness-check")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := switchexhaustivenesscheck.DefaultOptions()
		if v, ok := c.Options["allowDefaultCaseForExhaustiveSwitch"].(bool); ok {
			opts.AllowDefaultCaseForExhaustiveSwitch = v
		}
		if v, ok := c.Options["requireDefaultForNonUnion"].(bool); ok {
			opts.RequireDefaultForNonUnion = v
		}
		if v, ok := c.Options["considerDefaultExhaustiveForUnions"].(bool); ok {
			opts.ConsiderDefaultExhaustiveForUnions = v
		}
		if v, ok := c.Options["defaultCaseCommentPattern"].(string); ok {
			opts.DefaultCaseCommentPattern = v
		}
		noUnchecked := strings.Contains(c.LanguageOptionsText, "noUncheckedIndexedAccess")
		actual, runErr := runCase(t, c.Code, opts, noUnchecked)
		if runErr != nil {
			failed++
			continue
		}
		expected := c.ExpectedErrorCount
		if c.Valid {
			expected = 0
		}
		if actual == expected {
			passed++
			continue
		}
		failed++
		valid := "invalid"
		if c.Valid { valid = "valid" }
		t.Logf("FAIL [%s #%d] exp=%d act=%d hasOpts=%v\n%s\n", valid, c.SourceIndex, expected, actual, c.HasOptions, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("typescript-eslint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
}

func runCase(t *testing.T, code string, opts switchexhaustivenesscheck.Options, noUnchecked bool) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigTemplate(noUnchecked)), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	os.WriteFile(filepath.Join(dir, "switch-exhaustiveness-check.ts"), []byte(fixtureNamespacedEnumModule), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{switchexhaustivenesscheck.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"switch-exhaustiveness-check": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "switch-exhaustiveness-check" {
			count++
		}
	}
	return count, nil
}
