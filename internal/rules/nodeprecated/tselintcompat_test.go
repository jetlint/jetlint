package nodeprecated_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nodeprecated"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "jsx": "preserve",
    "skipLibCheck": true
  },
  "include": ["case.tsx", "deprecated.ts", "mixed-enums-decl.ts", "deprecated.tsx", "node-assert.d.ts", "react.d.ts", "node-fs.d.ts", "jsx.d.ts"]
}`

// fixtureTsconfigNode16Body mimics the upstream
// `tsconfig.moduleResolution-node16.json` extension: node16 module
// resolution with commonjs-style esModuleInterop. This changes how
// `import('./deprecated.js')` is typed — the result becomes a wrapper
// whose `.default` is the module's runtime export rather than the
// deprecated `export default` itself, matching what typescript-eslint
// expects for the `await import('./X.js')` fixtures.
const fixtureTsconfigNode16Body = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "node16",
    "moduleResolution": "node16", "lib": ["es2022", "dom"],
    "jsx": "preserve",
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["case.tsx", "deprecated.ts", "mixed-enums-decl.ts", "deprecated.tsx", "node-assert.d.ts", "react.d.ts", "node-fs.d.ts", "jsx.d.ts"]
}`

// fixtureNodeFsStub mirrors the @types/node fs typings used by the
// upstream allow-by-package fixtures. Only `exists` carries the
// `@deprecated` tag because it's the only deprecation the test cases
// exercise.
const fixtureNodeFsStub = `declare module 'fs' {
  /** @deprecated Since v1.0.0 - Use {@link stat} or {@link access} instead. */
  export function exists(path: string, callback: (exists: boolean) => void): void;
  export function stat(path: string): void;
  export function access(path: string): void;
}
`

// fixtureNodeAssertStub provides just enough of node:assert's typing
// surface to exercise the deprecated `fail(actual, expected, ...)`
// overload. typescript-eslint's tests resolve this via real @types/node;
// stubbing keeps the harness self-contained.
// fixtureReactStub provides the minimal React surface the JSX deprecation
// fixtures need — `React.FC<Props>` and a `<div>` intrinsic — so cases
// using component types resolve without bundling real @types/react.
const fixtureReactStub = `declare module 'react' {
  export type FC<P = {}> = (props: P) => any;
}
`

// fixtureJsxStub is a global (non-module) ambient script that declares
// the JSX namespace used when a case writes JSX without importing
// React. Putting this in a separate ambient file (no top-level
// import/export) means the `namespace JSX` augments the global scope,
// matching how @types/react contributes intrinsic-element typings.
// `div` carries the deprecated ARIA-1.1 `aria-grabbed` attribute that
// upstream's `<div aria-grabbed>` fixture exercises; other intrinsic
// elements stay any-shaped via the index signature.
const fixtureJsxStub = `declare namespace JSX {
  interface IntrinsicElements {
    [name: string]: any;
    div: {
      /** @deprecated in ARIA 1.1 */
      'aria-grabbed'?: boolean | 'true' | 'false';
      children?: any;
    };
  }
}
`

const fixtureNodeAssertStub = `declare module 'node:assert' {
  function fail(message?: string | Error): never;
  /** @deprecated since v10.0.0 - use fail([message]) or other assert functions instead. */
  function fail(actual: unknown, expected: unknown, message?: string | Error, operator?: string): never;
  const assertNs: { fail: typeof fail };
  export default assertNs;
}
`

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
		actual, runErr := runCase(t, c.Code, optsFromCase(c), tsconfigForCase(c))
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

func optsFromCase(c tselintcompat.Case) nodeprecated.Options {
	opts := nodeprecated.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	raw, ok := c.Options["allow"].([]any)
	if !ok {
		return opts
	}
	opts.AllowNames = map[string]struct{}{}
	for _, entry := range raw {
		switch v := entry.(type) {
		case string:
			opts.AllowNames[v] = struct{}{}
		case map[string]any:
			name, _ := v["name"].(string)
			if name == "" {
				continue
			}
			from, _ := v["from"].(string)
			pkg, _ := v["package"].(string)
			if from == "package" && pkg != "" {
				opts.AllowPackageNames = append(opts.AllowPackageNames, nodeprecated.AllowPackageEntry{Name: name, Package: pkg})
				continue
			}
			opts.AllowNames[name] = struct{}{}
		}
	}
	return opts
}

// tsconfigForCase picks the tsconfig body that matches the case's
// declared module-resolution preference. Cases whose
// `languageOptions.parserOptions.project` names the
// `tsconfig.moduleResolution-node16.json` override get the
// node16+esModuleInterop body so dynamic-import typing matches what
// the upstream test suite expects.
func tsconfigForCase(c tselintcompat.Case) string {
	if strings.Contains(c.LanguageOptionsText, "moduleResolution-node16") {
		return fixtureTsconfigNode16Body
	}
	return fixtureTsconfigBody
}

func runCase(t *testing.T, code string, opts nodeprecated.Options, tsconfigBody string) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(tsconfigBody), 0o644)
	os.WriteFile(filepath.Join(dir, "case.tsx"), []byte(code), 0o644)
	os.WriteFile(filepath.Join(dir, "deprecated.ts"), []byte(fixtureDeprecatedModule), 0o644)
	os.WriteFile(filepath.Join(dir, "node-assert.d.ts"), []byte(fixtureNodeAssertStub), 0o644)
	os.WriteFile(filepath.Join(dir, "react.d.ts"), []byte(fixtureReactStub), 0o644)
	os.WriteFile(filepath.Join(dir, "node-fs.d.ts"), []byte(fixtureNodeFsStub), 0o644)
	os.WriteFile(filepath.Join(dir, "jsx.d.ts"), []byte(fixtureJsxStub), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nodeprecated.NewWithOptions(opts)},
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
