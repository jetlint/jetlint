package preferreadonlyparametertypes_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/preferreadonlyparametertypes"
	"github.com/tommymorgan/tsgolint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts"]
}`

func TestPreferReadonlyParameterTypes_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/prefer-readonly-parameter-types.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "prefer-readonly-parameter-types")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := optsFromCase(c)
		actual, runErr := runCase(t, c.Code, opts)
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

func optsFromCase(c tselintcompat.Case) preferreadonlyparametertypes.Options {
	opts := preferreadonlyparametertypes.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["treatMethodsAsReadonly"].(bool); ok {
		opts.TreatMethodsAsReadonly = v
	}
	if v, ok := c.Options["ignoreInferredTypes"].(bool); ok {
		opts.IgnoreInferredTypes = v
	}
	return opts
}

func runCase(t *testing.T, code string, opts preferreadonlyparametertypes.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigBody), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{preferreadonlyparametertypes.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"prefer-readonly-parameter-types": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "prefer-readonly-parameter-types" {
			count++
		}
	}
	return count, nil
}
