package prefernullishcoalescing_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/prefernullishcoalescing"
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

// unstrictFixtureTsconfigBody mirrors typescript-eslint's `unstrict`
// fixture project — strict mode disabled with strictNullChecks
// explicitly off. Used when a case opts in via
// `languageOptions.parserOptions.tsconfigRootDir`.
const unstrictFixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": false, "strictNullChecks": false,
    "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts"]
}`

func TestPreferNullishCoalescing_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/prefer-nullish-coalescing.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "prefer-nullish-coalescing")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := optsFromCase(c)
		actual, runErr := runCase(t, c.Code, opts, c.Unstrict)
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

func optsFromCase(c tselintcompat.Case) prefernullishcoalescing.Options {
	opts := prefernullishcoalescing.DefaultOptions()
	if v, ok := c.Options["ignoreConditionalTests"].(bool); ok {
		opts.IgnoreConditionalTests = v
	}
	if v, ok := c.Options["ignoreMixedLogicalExpressions"].(bool); ok {
		opts.IgnoreMixedLogicalExpressions = v
	}
	if v, ok := c.Options["ignoreBooleanCoercion"].(bool); ok {
		opts.IgnoreBooleanCoercion = v
	}
	if v, ok := c.Options["ignoreTernaryTests"].(bool); ok {
		opts.IgnoreTernaryTests = v
	}
	if v, ok := c.Options["ignoreIfStatements"].(bool); ok {
		opts.IgnoreIfStatements = v
	}
	switch p := c.Options["ignorePrimitives"].(type) {
	case bool:
		if p {
			opts.IgnorePrimitives.Boolean = true
			opts.IgnorePrimitives.BigInt = true
			opts.IgnorePrimitives.Number = true
			opts.IgnorePrimitives.String = true
		}
	case map[string]interface{}:
		if v, ok := p["boolean"].(bool); ok {
			opts.IgnorePrimitives.Boolean = v
		}
		if v, ok := p["bigint"].(bool); ok {
			opts.IgnorePrimitives.BigInt = v
		}
		if v, ok := p["number"].(bool); ok {
			opts.IgnorePrimitives.Number = v
		}
		if v, ok := p["string"].(bool); ok {
			opts.IgnorePrimitives.String = v
		}
	}
	return opts
}

func runCase(t *testing.T, code string, opts prefernullishcoalescing.Options, unstrict bool) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	body := fixtureTsconfigBody
	if unstrict {
		body = unstrictFixtureTsconfigBody
	}
	os.WriteFile(tsc, []byte(body), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{prefernullishcoalescing.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"prefer-nullish-coalescing": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "prefer-nullish-coalescing" {
			count++
		}
	}
	return count, nil
}
