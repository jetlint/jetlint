package preferpromiserejecterrors_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/preferpromiserejecterrors"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true,
    "paths": { "errors": ["./errors.d.ts"] }
  },
  "include": ["case.ts", "errors.d.ts"]
}`

// fixtureErrorsModule stubs the upstream `errors` virtual module that a
// few test cases import from. The shape mirrors the upstream fixture in
// packages/eslint-plugin/tests/fixtures/errors/index.d.ts so cases that
// configure `allow: [{ from: 'package', name: 'ErrorLike', package: 'errors' }]`
// can match `createError()`'s return-type symbol name.
const fixtureErrorsModule = `export interface ErrorLike { stack?: string; message: string; }
export declare function createError(): ErrorLike;
`

func TestPreferPromiseRejectErrors_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/prefer-promise-reject-errors.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "prefer-promise-reject-errors")
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

func optsFromCase(c tselintcompat.Case) preferpromiserejecterrors.Options {
	opts := preferpromiserejecterrors.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["allowEmptyReject"].(bool); ok {
		opts.AllowEmptyReject = v
	}
	if v, ok := c.Options["allowThrowingAny"].(bool); ok {
		opts.AllowThrowingAny = v
	}
	if v, ok := c.Options["allowThrowingUnknown"].(bool); ok {
		opts.AllowThrowingUnknown = v
	}
	if raw, ok := c.Options["allow"].([]any); ok {
		for _, e := range raw {
			switch v := e.(type) {
			case string:
				opts.Allow = append(opts.Allow, preferpromiserejecterrors.TypeMatcher{Name: v})
			case map[string]any:
				m := preferpromiserejecterrors.TypeMatcher{}
				if s, ok := v["from"].(string); ok {
					m.From = s
				}
				if s, ok := v["name"].(string); ok {
					m.Name = s
				}
				if s, ok := v["package"].(string); ok {
					m.Package = s
				}
				if m.Name != "" {
					opts.Allow = append(opts.Allow, m)
				}
			}
		}
	}
	return opts
}

func runCase(t *testing.T, code string, opts preferpromiserejecterrors.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigBody), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	os.WriteFile(filepath.Join(dir, "errors.d.ts"), []byte(fixtureErrorsModule), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{preferpromiserejecterrors.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"prefer-promise-reject-errors": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "prefer-promise-reject-errors" {
			count++
		}
	}
	return count, nil
}
