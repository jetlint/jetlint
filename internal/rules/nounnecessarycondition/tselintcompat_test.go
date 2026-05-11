package nounnecessarycondition_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarycondition"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

// fixtureTsconfigTemplate generates the per-case tsconfig. Cases
// opt into `noUncheckedIndexedAccess` or the unstrict mode by their
// `languageOptions` text — the harness flips the corresponding
// compiler options when those markers appear.
func fixtureTsconfigTemplate(noUnchecked, unstrict bool) string {
	strict := "true"
	extraOpts := ""
	if noUnchecked {
		extraOpts += `, "noUncheckedIndexedAccess": true`
	}
	if unstrict {
		strict = "false"
	}
	return fmt.Sprintf(`{
  "compilerOptions": {
    "strict": %s, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true%s
  },
  "include": ["case.ts"]
}`, strict, extraOpts)
}

func TestNoUnnecessaryCondition_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/no-unnecessary-condition.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "no-unnecessary-condition")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := nounnecessarycondition.DefaultOptions()
		switch v := c.Options["allowConstantLoopConditions"].(type) {
		case bool:
			if v {
				opts.AllowConstantLoopConditions = nounnecessarycondition.LoopConditionAlways
			} else {
				opts.AllowConstantLoopConditions = nounnecessarycondition.LoopConditionNever
			}
		case string:
			switch v {
			case "always":
				opts.AllowConstantLoopConditions = nounnecessarycondition.LoopConditionAlways
			case "only-allowed-literals":
				opts.AllowConstantLoopConditions = nounnecessarycondition.LoopConditionOnlyAllowedLiterals
			case "never":
				opts.AllowConstantLoopConditions = nounnecessarycondition.LoopConditionNever
			}
		}
		if v, ok := c.Options["checkTypePredicates"].(bool); ok {
			opts.CheckTypePredicates = v
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

func runCase(t *testing.T, c tselintcompat.Case, opts nounnecessarycondition.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	noUnchecked := strings.Contains(strings.ToLower(c.LanguageOptionsText), strings.ToLower("noUncheckedIndexedAccess"))
	unstrict := c.Unstrict
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigTemplate(noUnchecked, unstrict)), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(c.Code), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nounnecessarycondition.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-unnecessary-condition": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-unnecessary-condition" {
			count++
		}
	}
	return count, nil
}
