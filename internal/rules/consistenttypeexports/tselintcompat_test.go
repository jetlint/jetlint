package consistenttypeexports_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/consistenttypeexports"
	"github.com/tommymorgan/tsgolint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts", "consistent-type-exports.ts"]
}`

// fixtureConsistentTypeExportsModule mirrors the upstream
// packages/eslint-plugin/tests/fixtures/consistent-type-exports/index.ts
// fixture so cases that re-export from './consistent-type-exports'
// can resolve symbols and the rule can classify each export as a
// type or value.
const fixtureConsistentTypeExportsModule = `export type Type1 = 1;
export type Type2 = 1;
export const value1 = 2;
export const value2 = 2;

export class Class1 {}

export type NAME = 'name';
export const NAME = 'name';
`

func TestConsistentTypeExports_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/consistent-type-exports.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "consistent-type-exports")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		actual, runErr := runCase(t, c.Code)
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

func runCase(t *testing.T, code string) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigBody), 0o644)
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	os.WriteFile(filepath.Join(dir, "consistent-type-exports.ts"), []byte(fixtureConsistentTypeExportsModule), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{consistenttypeexports.New()},
		map[string]wrapperlint.Severity{"consistent-type-exports": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "consistent-type-exports" {
			count++
		}
	}
	return count, nil
}
