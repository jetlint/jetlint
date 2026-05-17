package noreexportall_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/noreexportall"
)

const eslintTsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false,
    "jsx": "preserve"
  },
  "include": ["case.ts", "case.tsx"]
}`

func TestEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/no-re-export-all.json")
	fx, err := eslintcompat.Load(fixturePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for i, c := range fx.Cases {
		count, runErr := runEslintCase(t, c.Code)
		if runErr != nil {
			failed++
			t.Logf("FAIL [#%d] runCase: %v\n--- code ---\n%s\n--- end ---", i, runErr, c.Code)
			continue
		}
		ok := (c.Valid && count == 0) || (!c.Valid && count >= 1)
		if ok {
			passed++
			continue
		}
		failed++
		valid := "fail"
		if c.Valid {
			valid = "pass"
		}
		t.Logf("MISMATCH [%s #%d] count=%d\n--- code ---\n%s\n--- end ---",
			valid, i, count, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("biome compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
	if failed > 0 {
		t.Fatalf("expected 100%% pass rate, got %d/%d (%.1f%%)", passed, total, pct)
	}
}

func runEslintCase(t *testing.T, code string) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-nrx-compat")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(eslintTsconfigBody), 0o644); err != nil {
		return 0, err
	}
	// Detect JSX via heuristic and use .tsx when needed.
	src := "case.ts"
	if containsJSX(code) {
		src = "case.tsx"
	}
	if err := os.WriteFile(filepath.Join(dir, src), []byte(code), 0o644); err != nil {
		return 0, err
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{noreexportall.New()},
		map[string]wrapperlint.Severity{"no-re-export-all": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-re-export-all" {
			count++
		}
	}
	return count, nil
}

func containsJSX(s string) bool {
	// rough: a "<X" where X is a letter or fragment "<>"
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '<' {
			c := s[i+1]
			if c == '>' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				return true
			}
		}
	}
	return false
}
