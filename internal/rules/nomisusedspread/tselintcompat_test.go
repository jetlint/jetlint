package nomisusedspread_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nomisusedspread"
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

func TestNoMisusedSpread_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/no-misused-spread.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "no-misused-spread")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		opts := nomisusedspread.DefaultOptions()
		if v, ok := c.Options["allow"].([]any); ok {
			for _, a := range v {
				if s, ok := a.(string); ok {
					opts.Allow = append(opts.Allow, s)
					continue
				}
				if m, ok := a.(map[string]any); ok {
					if name, ok := m["name"].(string); ok {
						opts.Allow = append(opts.Allow, name)
					}
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

// declareModulePattern matches `declare module '<name>' { ... }`
// blocks. Used to lift ambient module declarations into stub
// .d.ts files under node_modules/<name>/ so subsequent `import
// { Foo } from '<name>'` statements in the same case can resolve.
var declareModulePattern = regexp.MustCompile(`(?s)declare\s+module\s+['"]([^'"]+)['"]\s*\{(.*?)\n\s*\}`)

func runCase(t *testing.T, code string, opts nomisusedspread.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigBody), 0o644)
	// Extract ambient `declare module '<name>'` blocks into separate
	// stub package files so `import` statements that reference them
	// resolve. Once moved out, the in-case block is removed.
	code = declareModulePattern.ReplaceAllStringFunc(code, func(match string) string {
		groups := declareModulePattern.FindStringSubmatch(match)
		if len(groups) != 3 {
			return match
		}
		name, body := groups[1], groups[2]
		pkgDir := filepath.Join(dir, "node_modules", name)
		os.MkdirAll(pkgDir, 0o755)
		os.WriteFile(filepath.Join(pkgDir, "index.d.ts"), []byte(body+"\n"), 0o644)
		return ""
	})
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nomisusedspread.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-misused-spread": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-misused-spread" {
			count++
		}
	}
	return count, nil
}
