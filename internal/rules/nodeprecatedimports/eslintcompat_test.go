package nodeprecatedimports_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/nodeprecatedimports"
)

const tsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false,
    "jsx": "preserve"
  },
  "include": ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx"]
}`

const ruleID = "no-deprecated-imports"

func TestEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/" + ruleID + ".json")
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
	t.Logf("biome compatibility: %d/%d passed", passed, total)
	if failed > 0 {
		t.Fatalf("expected 100%% pass rate, got %d/%d", passed, total)
	}
}

func runCase(t *testing.T, c eslintcompat.Case) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-ndi-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigBody), 0o644); err != nil {
		return 0, err
	}
	mainPath := ""
	if len(c.Files) > 0 {
		for _, f := range c.Files {
			if err := os.WriteFile(filepath.Join(dir, f.Path), []byte(f.Content), 0o644); err != nil {
				return 0, err
			}
		}
		mainPath = c.Main
		if mainPath == "" {
			mainPath = c.Files[0].Path
		}
	} else {
		mainPath = "case.ts"
		if err := os.WriteFile(filepath.Join(dir, mainPath), []byte(c.Code), 0o644); err != nil {
			return 0, err
		}
	}
	prog, err := wrapperchecker.LoadProgram(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nodeprecatedimports.New()},
		map[string]wrapperlint.Severity{ruleID: wrapperlint.SeverityError},
	)
	count := 0
	wantPath := filepath.Join(dir, mainPath)
	for _, d := range eng.Lint(prog) {
		if d.RuleID != ruleID {
			continue
		}
		if d.Range.File != wantPath {
			continue
		}
		count++
	}
	return count, nil
}
