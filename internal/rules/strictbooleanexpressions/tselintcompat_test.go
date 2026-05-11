package strictbooleanexpressions_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/strictbooleanexpressions"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

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

func TestStrictBooleanExpressions_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/strict-boolean-expressions.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "strict-boolean-expressions")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := optsFromCase(c)
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
		if c.Valid {
			valid = "valid"
		}
		t.Logf("FAIL [%s #%d] exp=%d act=%d hasOpts=%v\n%s\n", valid, c.SourceIndex, expected, actual, c.HasOptions, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("typescript-eslint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
}

func optsFromCase(c tselintcompat.Case) strictbooleanexpressions.Options {
	opts := strictbooleanexpressions.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["allowString"].(bool); ok {
		opts.AllowString = v
	}
	if v, ok := c.Options["allowNumber"].(bool); ok {
		opts.AllowNumber = v
	}
	if v, ok := c.Options["allowNullableObject"].(bool); ok {
		opts.AllowNullableObject = v
	}
	if v, ok := c.Options["allowNullableBoolean"].(bool); ok {
		opts.AllowNullableBoolean = v
	}
	if v, ok := c.Options["allowNullableString"].(bool); ok {
		opts.AllowNullableString = v
	}
	if v, ok := c.Options["allowNullableNumber"].(bool); ok {
		opts.AllowNullableNumber = v
	}
	if v, ok := c.Options["allowNullableEnum"].(bool); ok {
		opts.AllowNullableEnum = v
	}
	if v, ok := c.Options["allowAny"].(bool); ok {
		opts.AllowAny = v
	}
	if v, ok := c.Options["allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing"].(bool); ok {
		opts.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing = v
	}
	return opts
}

func runCase(t *testing.T, c tselintcompat.Case, opts strictbooleanexpressions.Options) (int, error) {
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
		[]engine.Rule{strictbooleanexpressions.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"strict-boolean-expressions": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "strict-boolean-expressions" {
			count++
		}
	}
	return count, nil
}
