package nounnecessarybooleanliteralcompare_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarybooleanliteralcompare"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

// fixtureTsconfigTemplate generates the per-case tsconfig. The single
// case that needs `strictNullChecks: false` flips `strict` off.
func fixtureTsconfigTemplate(unstrict bool) string {
	strict := "true"
	if unstrict {
		strict = "false"
	}
	return fmt.Sprintf(`{
  "compilerOptions": {
    "strict": %s, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts"]
}`, strict)
}

func TestNoUnnecessaryBooleanLiteralCompare_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/no-unnecessary-boolean-literal-compare.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "no-unnecessary-boolean-literal-compare")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := nounnecessarybooleanliteralcompare.DefaultOptions()
		if v, ok := c.Options["allowComparingNullableBooleansToTrue"].(bool); ok {
			opts.AllowComparingNullableBooleansToTrue = v
		}
		if v, ok := c.Options["allowComparingNullableBooleansToFalse"].(bool); ok {
			opts.AllowComparingNullableBooleansToFalse = v
		}
		if v, ok := c.Options["allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing"].(bool); ok {
			opts.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing = v
		}
		actual, runErr := runCase(t, c, opts)
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

func runCase(t *testing.T, c tselintcompat.Case, opts nounnecessarybooleanliteralcompare.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigTemplate(c.Unstrict)), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(c.Code), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nounnecessarybooleanliteralcompare.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-unnecessary-boolean-literal-compare": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-unnecessary-boolean-literal-compare" {
			count++
		}
	}
	return count, nil
}
