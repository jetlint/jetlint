package noundef_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/eslintcompat"
	"github.com/jetlint/jetlint/internal/rules/noundef"
)

// eslintTsconfigBody includes the `dom` lib so the global names used
// by oxc's fixtures (URLSearchParams, IntersectionObserver, …) are
// considered declared without our extractor needing to capture the
// `env: { browser: true }` setting from the upstream test source.
// `esnext` is used (not `es2022`) so newer built-ins like
// `Float16Array` and `Iterator` resolve to library declarations.
// tsconfigBodyForCase tailors the TypeScript lib list to the
// fixture's per-case `env` setting: oxc tests opt into browser
// globals via `env: { browser: true }`, which we reflect by adding
// the `dom` lib. Cases without that setting see only the plain
// `esnext` lib, so genuinely-missing globals like `window` are
// flagged the same way oxc flags them.
func tsconfigBodyForCase(c eslintcompat.Case) string {
	lib := `"esnext"`
	if env, _ := c.Settings["env"].(map[string]any); env != nil {
		if v, _ := env["browser"].(bool); v {
			lib = `"esnext", "dom"`
		}
	}
	return `{
  "compilerOptions": {
    "strict": false, "target": "esnext", "module": "esnext",
    "moduleResolution": "bundler", "lib": [` + lib + `],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false
  },
  "include": ["case.tsx", "globals.d.ts"]
}`
}

func TestNoUndef_EslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/eslint/no-undef.json")
	fx, err := eslintcompat.Load(fixturePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for i, c := range fx.Cases {
		count, runErr := runEslintCase(t, c)
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
}

func runEslintCase(t *testing.T, c eslintcompat.Case) (int, error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "jl-nu-undef")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(tsconfigBodyForCase(c)), 0o644); err != nil {
		return 0, err
	}
	// Synthesise an ambient declaration file so each global named by
	// the fixture's `globals` settings is recognised as declared (the
	// no-undef rule sees a resolved symbol and skips reporting).
	if names := globalNamesFromCase(c); len(names) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "globals.d.ts"),
			[]byte(globalsAmbientFile(names)), 0o644); err != nil {
			return 0, err
		}
	}
	// Allow JSX since several oxc fixtures use it.
	if err := os.WriteFile(filepath.Join(dir, "case.tsx"), []byte(c.Code), 0o644); err != nil {
		return 0, err
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{noundef.New()},
		map[string]wrapperlint.Severity{"no-undef": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-undef" {
			count++
		}
	}
	return count, nil
}

// globalNamesFromCase returns the names listed under the fixture's
// `globals` settings — values may be bool or strings (ESLint accepts
// either; we only care about the name).
func globalNamesFromCase(c eslintcompat.Case) []string {
	g := c.Globals()
	if len(g) == 0 {
		return nil
	}
	names := make([]string, 0, len(g))
	for name := range g {
		names = append(names, name)
	}
	return names
}

func globalsAmbientFile(names []string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("declare var ")
		b.WriteString(n)
		b.WriteString(": any;\n")
	}
	return b.String()
}

var _ = eslintcompat.Case{}
