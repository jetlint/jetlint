package nobasetotostring_test

// Compatibility harness: runs every case from typescript-eslint's
// no-base-to-string.test.ts through our rule and reports
// pass/fail counts. Skipped under -short.

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/rules/nobasetotostring"
	"github.com/tommymorgan/tsgolint/internal/tselintcompat"
)

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

func TestNoBaseToString_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}

	fixturePath, err := filepath.Abs("../../../testdata/typescript-eslint/no-base-to-string.test.ts")
	if err != nil {
		t.Fatalf("abs fixture path: %v", err)
	}
	cases, err := tselintcompat.Load(fixturePath, "no-base-to-string")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	var passed, failed int
	for _, c := range cases {
		actual, runErr := runCase(t, c.Code, optionsFromCase(c))
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

func optionsFromCase(c tselintcompat.Case) nobasetotostring.Options {
	opts := nobasetotostring.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["checkUnknown"].(bool); ok {
		opts.CheckUnknown = v
	}
	if arr, ok := c.Options["ignoredTypeNames"].([]any); ok {
		out := make([]string, 0, len(arr))
		for _, e := range arr {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		opts.IgnoredTypeNames = out
	}
	return opts
}

func runCase(t *testing.T, code string, opts nobasetotostring.Options) (int, error) {
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
		[]engine.Rule{nobasetotostring.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-base-to-string": wrapperlint.SeverityError},
	)
	diags := eng.Lint(prog)
	count := 0
	for _, d := range diags {
		if d.RuleID == "no-base-to-string" {
			count++
		}
	}
	return count, nil
}
