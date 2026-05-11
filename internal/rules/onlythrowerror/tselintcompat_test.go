package onlythrowerror_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/onlythrowerror"
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

func TestOnlyThrowError_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/only-throw-error.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "only-throw-error")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		actual, runErr := runCase(t, c.Code, optionsFromCase(c))
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
		t.Logf("FAIL [%s #%d] expected=%d actual=%d hasOptions=%v\n--- code ---\n%s\n--- end ---", valid, c.SourceIndex, expected, actual, c.HasOptions, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("typescript-eslint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
}

func optionsFromCase(c tselintcompat.Case) onlythrowerror.Options {
	opts := onlythrowerror.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["allowThrowingAny"].(bool); ok {
		opts.AllowThrowingAny = v
	}
	if v, ok := c.Options["allowThrowingUnknown"].(bool); ok {
		opts.AllowThrowingUnknown = v
	}
	if v, ok := c.Options["allowRethrowing"].(bool); ok {
		opts.AllowRethrowing = v
	}
	if arr, ok := c.Options["allow"].([]any); ok {
		out := make([]onlythrowerror.TypeMatcher, 0, len(arr))
		for _, e := range arr {
			switch x := e.(type) {
			case string:
				if x != "" {
					out = append(out, onlythrowerror.TypeMatcher{Name: x})
				}
			case map[string]any:
				from, _ := x["from"].(string)
				name, _ := x["name"].(string)
				if name != "" {
					out = append(out, onlythrowerror.TypeMatcher{From: from, Name: name})
				}
			}
		}
		opts.Allow = out
	}
	return opts
}

func runCase(t *testing.T, code string, opts onlythrowerror.Options) (int, error) {
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
		[]engine.Rule{onlythrowerror.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"only-throw-error": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "only-throw-error" {
			count++
		}
	}
	return count, nil
}
