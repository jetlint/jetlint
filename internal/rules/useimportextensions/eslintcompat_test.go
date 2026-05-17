package useimportextensions_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/useimportextensions"
)

const tsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "commonjs",
    "moduleResolution": "node", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false,
    "jsx": "preserve"
  },
  "include": ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx", "**/*.d.ts"]
}`

func TestUseImportExtensions_EslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/use-import-extensions.json")
	fx, err := eslintcompat.Load(fixturePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for i, c := range fx.Cases {
		count, runErr := runCase(t, c)
		if runErr != nil {
			failed++
			t.Logf("FAIL [#%d] runCase: %v", i, runErr)
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
		t.Logf("MISMATCH [%s #%d] count=%d", valid, i, count)
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

func runCase(t *testing.T, c eslintcompat.Case) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-uie-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigBody), 0o644); err != nil {
		return 0, err
	}
	mainPath := c.Main
	for _, f := range c.Files {
		full := filepath.Join(dir, f.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return 0, err
		}
		if err := os.WriteFile(full, []byte(f.Content), 0o644); err != nil {
			return 0, err
		}
	}
	if mainPath == "" && len(c.Files) > 0 {
		mainPath = c.Files[0].Path
	}
	prog, err := wrapperchecker.LoadProgram(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{useimportextensions.New()},
		map[string]wrapperlint.Severity{"use-import-extensions": wrapperlint.SeverityError},
	)
	count := 0
	wantPath := filepath.Join(dir, mainPath)
	for _, d := range eng.Lint(prog) {
		if d.RuleID != "use-import-extensions" {
			continue
		}
		if d.Range.File != wantPath {
			continue
		}
		count++
	}
	return count, nil
}
