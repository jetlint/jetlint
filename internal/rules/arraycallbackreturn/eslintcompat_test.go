package arraycallbackreturn_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/arraycallbackreturn"
)

const eslintTsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false
  },
  "include": ["case.ts"]
}`

func TestArrayCallbackReturn_EslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/array-callback-return.json")
	fx, err := eslintcompat.Load(fixturePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for i, c := range fx.Cases {
		opts := optionsForCase(c.Options)
		count, runErr := runEslintCase(t, c.Code, opts)
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
		t.Logf("MISMATCH [%s #%d opts=%+v] count=%d\n--- code ---\n%s\n--- end ---",
			valid, i, opts, count, c.Code)
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("oxlint compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
}

// optionsForCase converts the oxlint case options array to jetlint
// Options. Oxlint cases store options as `[options0]` where options0
// is a JSON object like `{"allowImplicit": true, "checkForEach": true}`.
func optionsForCase(rawOptions []any) arraycallbackreturn.Options {
	if len(rawOptions) == 0 {
		return arraycallbackreturn.Options{}
	}
	body, err := json.Marshal(rawOptions[0])
	if err != nil {
		return arraycallbackreturn.Options{}
	}
	opts, err := arraycallbackreturn.OptionsFromJSON(body)
	if err != nil {
		return arraycallbackreturn.Options{}
	}
	return opts
}

func runEslintCase(t *testing.T, code string, opts arraycallbackreturn.Options) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-acr-compat")
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
		[]engine.Rule{arraycallbackreturn.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"array-callback-return": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "array-callback-return" {
			count++
		}
	}
	return count, nil
}
