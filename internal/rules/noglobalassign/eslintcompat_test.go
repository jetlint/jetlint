package noglobalassign_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/noglobalassign"
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

const ruleID = "no-global-assign"

func TestEslintCompatibility(t *testing.T) {
	if testing.Short() { t.Skip("compatibility harness skipped under -short") }
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/" + ruleID + ".json")
	fx, err := eslintcompat.Load(fixturePath)
	if err != nil { t.Fatalf("load: %v", err) }
	var passed, failed int
	for i, c := range fx.Cases {
		count, runErr := runEslintCase(t, c.Code)
		if runErr != nil { failed++; t.Logf("FAIL [#%d] %v\n%s", i, runErr, c.Code); continue }
		ok := (c.Valid && count == 0) || (!c.Valid && count >= 1)
		if ok { passed++; continue }
		failed++
		valid := "fail"
		if c.Valid { valid = "pass" }
		t.Logf("MISMATCH [%s #%d] count=%d\n%s", valid, i, count, c.Code)
	}
	if failed > 0 { t.Fatalf("expected 100%% pass rate, got %d/%d", passed, passed+failed) }
}

func runEslintCase(t *testing.T, code string) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-")
	if err != nil { return 0, err }
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(eslintTsconfigBody), 0o644)
	src := "case.ts"
	if containsJSX(code) { src = "case.tsx" }
	os.WriteFile(filepath.Join(dir, src), []byte(code), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil { return 0, err }
	defer prog.Close()
	eng := engine.New([]engine.Rule{noglobalassign.New()}, map[string]wrapperlint.Severity{ruleID: wrapperlint.SeverityError})
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == ruleID { count++ }
	}
	return count, nil
}

func containsJSX(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '<' {
			c := s[i+1]
			if c == '>' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') { return true }
		}
	}
	return false
}
