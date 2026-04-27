package nofloatingpromises_test

// Compatibility harness: runs every case from typescript-eslint's
// no-floating-promises.test.ts through our rule and reports
// pass/fail counts. Skipped under -short.
//
// A "pass" means our rule's diagnostic count for the fixture matches
// typescript-eslint's:
//   - valid case: 0 diagnostics
//   - invalid case: exactly ExpectedErrorCount diagnostics
//
// A "fail" means the count diverged. We log every divergence so the
// list of discrepancies is the spike's actual output.

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/nofloatingpromises"
	"github.com/tommymorgan/tsgolint/internal/tselintcompat"
)

// fixtureTsconfigBody mirrors the typescript-eslint fixtures tsconfig:
// strict mode, modern lib, but without type packages we don't have
// available locally (node, react). The cases under test don't depend
// on those packages.
const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true,
    "target": "es2022",
    "module": "esnext",
    "moduleResolution": "bundler",
    "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts"]
}`

func TestNoFloatingPromises_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}

	fixturePath, err := filepath.Abs("../../../testdata/typescript-eslint/no-floating-promises.test.ts")
	if err != nil {
		t.Fatalf("abs fixture path: %v", err)
	}
	cases, err := tselintcompat.Load(fixturePath, "no-floating-promises")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	var passed, failed int
	for _, c := range cases {
		opts := optionsFromCase(c)
		actual, runErr := runCase(t, c.Code, opts)
		if runErr != nil {
			failed++
			t.Logf("FAIL [%s #%d]: load error: %v", labelFor(c.Valid), c.SourceIndex, runErr)
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
		t.Logf("FAIL [%s #%d] expected=%d actual=%d hasOptions=%v\n--- code ---\n%s\n--- end ---",
			labelFor(c.Valid), c.SourceIndex, expected, actual, c.HasOptions, c.Code)
	}

	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("typescript-eslint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
}

func labelFor(valid bool) string {
	if valid {
		return "valid"
	}
	return "invalid"
}

// optionsFromCase translates the loader's parsed `options[0]` map into
// a typed Options struct. Defaults track typescript-eslint's defaults:
// IgnoreVoid=true, all others zero-valued.
func optionsFromCase(c tselintcompat.Case) nofloatingpromises.Options {
	opts := nofloatingpromises.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["ignoreVoid"].(bool); ok {
		opts.IgnoreVoid = v
	}
	if v, ok := c.Options["ignoreIIFE"].(bool); ok {
		opts.IgnoreIIFE = v
	}
	if v, ok := c.Options["checkThenables"].(bool); ok {
		opts.CheckThenables = v
	}
	opts.AllowForKnownSafePromises = matchersFromAny(c.Options["allowForKnownSafePromises"])
	opts.AllowForKnownSafeCalls = matchersFromAny(c.Options["allowForKnownSafeCalls"])
	return opts
}

// matchersFromAny accepts both shapes typescript-eslint allows in its
// schema: a bare string name (`'randomAsyncFunction'`) or an object
// (`{ from: 'file', name: 'Foo' }`).
func matchersFromAny(v any) []nofloatingpromises.TypeMatcher {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]nofloatingpromises.TypeMatcher, 0, len(arr))
	for _, e := range arr {
		switch x := e.(type) {
		case string:
			if x != "" {
				out = append(out, nofloatingpromises.TypeMatcher{Name: x})
			}
		case map[string]any:
			from, _ := x["from"].(string)
			name, _ := x["name"].(string)
			if name != "" {
				out = append(out, nofloatingpromises.TypeMatcher{From: from, Name: name})
			}
		}
	}
	return out
}

func runCase(t *testing.T, code string, opts nofloatingpromises.Options) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)

	tsconfig := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsconfig, []byte(fixtureTsconfigBody), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644); err != nil {
		return 0, err
	}

	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		return 0, err
	}
	defer prog.Close()

	eng := engine.New(
		[]engine.Rule{nofloatingpromises.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-floating-promises": wrapperlint.SeverityError},
	)
	diags := eng.Lint(prog)
	count := 0
	for _, d := range diags {
		if d.RuleID == "no-floating-promises" {
			count++
		}
	}
	return count, nil
}
