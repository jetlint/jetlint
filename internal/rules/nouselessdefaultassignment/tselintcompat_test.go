package nouselessdefaultassignment_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nouselessdefaultassignment"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

// fixtureTsconfigTemplate renders the tsconfig for a case. Cases whose
// `languageOptions.parserOptions.tsconfigRootDir` points at the
// `unstrict` fixture root mean the upstream test exercises the rule
// without strict null checks — flip the strict flag so our program
// matches that configuration.
func fixtureTsconfigTemplate(strict bool) string {
	strictStr := "true"
	if !strict {
		strictStr = "false"
	}
	return `{
  "compilerOptions": {
    "strict": ` + strictStr + `, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts"]
}`
}

func TestNoUselessDefaultAssignment_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/no-useless-default-assignment.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "no-useless-default-assignment")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		actual, runErr := runCase(t, c.Code, !c.Unstrict, optsFromCase(c))
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

func optsFromCase(c tselintcompat.Case) nouselessdefaultassignment.Options {
	opts := nouselessdefaultassignment.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing"].(bool); ok {
		opts.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing = v
	}
	return opts
}

func runCase(t *testing.T, code string, strict bool, opts nouselessdefaultassignment.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigTemplate(strict)), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nouselessdefaultassignment.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-useless-default-assignment": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-useless-default-assignment" {
			count++
		}
	}
	return count, nil
}
