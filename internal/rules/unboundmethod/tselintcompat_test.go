package unboundmethod_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/unboundmethod"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts", "class.ts"]
}`

// fixtureClassModule mirrors the upstream
// packages/eslint-plugin/tests/fixtures/class.ts so cases that import
// from './class' (a few unbound-method scenarios) resolve.
const fixtureClassModule = `// used by no-throw-literal test case to validate custom error
export class Error {}

// used by unbound-method test case to test imports
export const console = { log() {} };

// used by prefer-reduce-type-parameter to test native vs userland check
export class Reducable {
  reduce() {}
}

// used by no-implied-eval test function imports
export class Function {}
`

func TestUnboundMethod_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/unbound-method.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "unbound-method")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := unboundmethod.DefaultOptions()
		if v, ok := c.Options["ignoreStatic"].(bool); ok {
			opts.IgnoreStatic = v
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

func runCase(t *testing.T, code string, opts unboundmethod.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigBody), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	os.WriteFile(filepath.Join(dir, "class.ts"), []byte(fixtureClassModule), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{unboundmethod.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"unbound-method": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "unbound-method" {
			count++
		}
	}
	return count, nil
}
