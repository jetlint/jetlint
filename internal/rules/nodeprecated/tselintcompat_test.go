package nodeprecated_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/nodeprecated"
	"github.com/tommymorgan/tsgolint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "jsx": "preserve",
    "skipLibCheck": true
  },
  "include": ["case.tsx", "deprecated.ts", "mixed-enums-decl.ts", "deprecated.tsx"]
}`

// fixtureDeprecatedModule mirrors the upstream fixture at
// packages/eslint-plugin/tests/fixtures/deprecated.ts so cases that
// import from './deprecated' resolve.
const fixtureDeprecatedModule = `/** @deprecated */
export class DeprecatedClass {
  /** @deprecated */
  foo: string = '';
}
/** @deprecated */
export const deprecatedVariable = 1;
/** @deprecated */
export function deprecatedFunction(): void {}
class NormalClass {}
const normalVariable = 1;
function normalFunction(): void;
function normalFunction(arg: string): void;
function normalFunction(arg?: string): void {}
function deprecatedFunctionWithOverloads(): void;
/** @deprecated */
function deprecatedFunctionWithOverloads(arg: string): void;
function deprecatedFunctionWithOverloads(arg?: string): void {}
export class ClassWithDeprecatedConstructor {
  constructor();
  /** @deprecated */
  constructor(arg: string);
  constructor(arg?: string) {}
}
export {
  /** @deprecated */
  NormalClass,
  /** @deprecated */
  normalVariable,
  /** @deprecated */
  normalFunction,
  deprecatedFunctionWithOverloads,
  /** @deprecated Reason */
  deprecatedFunctionWithOverloads as reexportedDeprecatedFunctionWithOverloads,
  /** @deprecated Reason */
  ClassWithDeprecatedConstructor as ReexportedClassWithDeprecatedConstructor,
};

/** @deprecated Reason */
export type T = { a: string };

export type U = { b: string };

/** @deprecated */
export default {
  foo: 1,
};
`

func TestNoDeprecated_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/no-deprecated.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "no-deprecated")
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
	os.WriteFile(filepath.Join(dir, "case.tsx"), []byte(code), 0o644)
	os.WriteFile(filepath.Join(dir, "deprecated.ts"), []byte(fixtureDeprecatedModule), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nodeprecated.New()},
		map[string]wrapperlint.Severity{"no-deprecated": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-deprecated" {
			count++
		}
	}
	return count, nil
}
