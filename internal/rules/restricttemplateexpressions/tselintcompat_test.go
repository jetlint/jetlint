package restricttemplateexpressions_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/restricttemplateexpressions"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts"]
}`

func TestRestrictTemplateExpressions_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/restrict-template-expressions.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "restrict-template-expressions")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := optionsFromCase(c)
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

func optionsFromCase(c tselintcompat.Case) restricttemplateexpressions.Options {
	opts := restricttemplateexpressions.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["allowNumber"].(bool); ok {
		opts.AllowNumber = v
	}
	if v, ok := c.Options["allowBoolean"].(bool); ok {
		opts.AllowBoolean = v
	}
	if v, ok := c.Options["allowAny"].(bool); ok {
		opts.AllowAny = v
	}
	if v, ok := c.Options["allowNullish"].(bool); ok {
		opts.AllowNullish = v
	}
	if v, ok := c.Options["allowRegExp"].(bool); ok {
		opts.AllowRegExp = v
	}
	if v, ok := c.Options["allowNever"].(bool); ok {
		opts.AllowNever = v
	}
	if v, ok := c.Options["allowArray"].(bool); ok {
		opts.AllowArray = v
	}
	if arr, ok := c.Options["allow"].([]any); ok {
		out := make([]restricttemplateexpressions.TypeMatcher, 0, len(arr))
		for _, e := range arr {
			switch x := e.(type) {
			case string:
				if x != "" {
					out = append(out, restricttemplateexpressions.TypeMatcher{Name: x})
				}
			case map[string]any:
				name, _ := x["name"].(string)
				from, _ := x["from"].(string)
				if name != "" {
					out = append(out, restricttemplateexpressions.TypeMatcher{Name: name, From: from})
				}
			}
		}
		opts.Allow = out
	}
	return opts
}

func runCase(t *testing.T, code string, opts restricttemplateexpressions.Options) (int, error) {
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
		[]engine.Rule{restricttemplateexpressions.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"restrict-template-expressions": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "restrict-template-expressions" {
			count++
		}
	}
	return count, nil
}
