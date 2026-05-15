package nodebugger_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/nodebugger"
)

const eslintTsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false
  },
  "include": ["case.ts"]
}`

func TestNoDebugger_EslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/no-debugger.json")
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
	t.Logf("oxlint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
	if failed > 0 {
		t.Fatalf("expected 100%% pass rate, got %d/%d (%.1f%%)", passed, total, pct)
	}
}

func runEslintCase(t *testing.T, code string) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-ndbg-compat")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(eslintTsconfigBody), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644); err != nil {
		return 0, err
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nodebugger.New()},
		map[string]wrapperlint.Severity{"no-debugger": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-debugger" {
			count++
		}
	}
	return count, nil
}
