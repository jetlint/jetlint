package promisefunctionasync_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/promisefunctionasync"
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

func TestPromiseFunctionAsync_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/promise-function-async.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "promise-function-async")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := promisefunctionasync.DefaultOptions()
		if v, ok := c.Options["allowAny"].(bool); ok {
			opts.AllowAny = v
		}
		if v, ok := c.Options["checkArrowFunctions"].(bool); ok {
			opts.CheckArrowFunctions = v
		}
		if v, ok := c.Options["checkFunctionDeclarations"].(bool); ok {
			opts.CheckFunctionDeclarations = v
		}
		if v, ok := c.Options["checkFunctionExpressions"].(bool); ok {
			opts.CheckFunctionExpressions = v
		}
		if v, ok := c.Options["checkMethodDeclarations"].(bool); ok {
			opts.CheckMethodDeclarations = v
		}
		if v, ok := c.Options["allowedPromiseNames"].([]any); ok {
			for _, raw := range v {
				if s, ok := raw.(string); ok {
					opts.AllowedPromiseNames = append(opts.AllowedPromiseNames, s)
				}
			}
		}
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
		if c.Valid {
			valid = "valid"
		}
		t.Logf("FAIL [%s #%d] exp=%d act=%d\n%s\n", valid, c.SourceIndex, expected, actual, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("typescript-eslint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
}

func runCase(t *testing.T, code string, opts promisefunctionasync.Options) (int, error) {
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
		[]engine.Rule{promisefunctionasync.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"promise-function-async": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "promise-function-async" {
			count++
		}
	}
	return count, nil
}
