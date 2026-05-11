package nomisusedpromises_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nomisusedpromises"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

const fixtureTsconfigBody = `{
  "compilerOptions": {
    "strict": true,
    "target": "es2022",
    "module": "esnext",
    "moduleResolution": "bundler",
    "lib": ["es2022", "dom"],
    "jsx": "preserve",
    "skipLibCheck": true
  },
  "include": ["case.tsx"]
}`

func TestNoMisusedPromises_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}

	fixturePath, err := filepath.Abs("../../../testdata/typescript-eslint/no-misused-promises.test.ts")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	cases, err := tselintcompat.Load(fixturePath, "no-misused-promises")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var passed, failed int
	for _, c := range cases {
		opts := optsFromCase(c)
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

func optsFromCase(c tselintcompat.Case) nomisusedpromises.Options {
	opts := nomisusedpromises.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if v, ok := c.Options["checksConditionals"].(bool); ok {
		opts.ChecksConditionals = v
	}
	if v, ok := c.Options["checksSpreads"].(bool); ok {
		opts.ChecksSpreads = v
	}
	switch v := c.Options["checksVoidReturn"].(type) {
	case bool:
		opts.ChecksVoidReturn = v
		if !v {
			opts.VoidReturn = nomisusedpromises.VoidReturnSubOptions{}
		}
	case map[string]any:
		opts.ChecksVoidReturn = true
		sub := make(map[string]bool, len(v))
		for k, val := range v {
			if b, ok := val.(bool); ok {
				sub[k] = b
			}
		}
		opts = nomisusedpromises.ApplyVoidReturnSubOptions(opts, sub)
	}
	return opts
}

func runCase(t *testing.T, code string, opts nomisusedpromises.Options) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(fixtureTsconfigBody), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "case.tsx"), []byte(code), 0o644); err != nil {
		return 0, err
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nomisusedpromises.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-misused-promises": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-misused-promises" {
			count++
		}
	}
	return count, nil
}
