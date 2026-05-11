package preferdestructuring_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/preferdestructuring"
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

func TestPreferDestructuring_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/prefer-destructuring.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "prefer-destructuring")
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

// optsFromCase parses typescript-eslint's two-slot option shape:
// `[ <per-pattern config>, <general flags> ]`. The first slot is either
// `{ array, object }` (applies to both contexts) or
// `{ VariableDeclarator: {...}, AssignmentExpression: {...} }`. The
// second slot is `{ enforceForRenamedProperties, enforceForDeclarationWithTypeAnnotation }`.
func optsFromCase(c tselintcompat.Case) preferdestructuring.Options {
	opts := preferdestructuring.DefaultOptions()
	if len(c.AllOptions) > 0 {
		if first, ok := c.AllOptions[0].(map[string]any); ok {
			if hasShared(first, "array") || hasShared(first, "object") {
				cfg := configFrom(first)
				opts.AssignmentExpression = cfg
				opts.VariableDeclarator = cfg
			} else {
				if v, ok := first["AssignmentExpression"].(map[string]any); ok {
					opts.AssignmentExpression = configFrom(v)
				}
				if v, ok := first["VariableDeclarator"].(map[string]any); ok {
					opts.VariableDeclarator = configFrom(v)
				}
			}
		}
	}
	if len(c.AllOptions) > 1 {
		if second, ok := c.AllOptions[1].(map[string]any); ok {
			if v, ok := second["enforceForDeclarationWithTypeAnnotation"].(bool); ok {
				opts.EnforceForDeclarationWithTypeAnnotation = v
			}
			if v, ok := second["enforceForRenamedProperties"].(bool); ok {
				opts.EnforceForRenamedProperties = v
			}
		}
	}
	return opts
}

func hasShared(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

// configFrom mirrors upstream: a destructuring-config object opts in
// explicitly. Keys absent from the option default to `false`, NOT to
// `true`. So `{ array: true }` enables array destructuring and leaves
// object destructuring disabled.
func configFrom(m map[string]any) preferdestructuring.DestructuringConfig {
	cfg := preferdestructuring.DestructuringConfig{}
	if v, ok := m["array"].(bool); ok {
		cfg.Array = v
	}
	if v, ok := m["object"].(bool); ok {
		cfg.Object = v
	}
	return cfg
}

func runCase(t *testing.T, code string, opts preferdestructuring.Options) (int, error) {
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
		[]engine.Rule{preferdestructuring.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"prefer-destructuring": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "prefer-destructuring" {
			count++
		}
	}
	return count, nil
}
