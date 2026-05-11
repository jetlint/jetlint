package nounnecessarytypeassertion_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeassertion"
	"github.com/jetlint/jetlint/internal/tselintcompat"
)

// hasJsxTag reports whether the source contains an opening JSX tag
// `<Tag ...>` or a self-closing JSX tag `<Tag ... />` where Tag is a
// JSX-style identifier (`div`, `Component`). Excludes TypeScript type
// assertions like `<number>(x)` which use the same `<` syntax.
func hasJsxTag(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		next := s[i+1]
		if !isJsxNameStart(next) {
			continue
		}
		// Scan the identifier portion; immediately after, expect either
		// `>` (open tag, including `/>`), whitespace+attr (`<Foo attr=`)
		// or `/>`. Type assertion `<number>(...)` has the form
		// `<Identifier>(` — distinguish via lookbehind: type assertions
		// follow an `=` or `(`/`,`, not text that ends a closing tag.
		end := i + 2
		for end < len(s) && (isJsxNameCont(s[end])) {
			end++
		}
		if end >= len(s) {
			continue
		}
		c := s[end]
		// JSX: `<Foo />` or `<Foo>` followed by a child or another `<`.
		if c == '/' && end+1 < len(s) && s[end+1] == '>' {
			// `<Foo/>` — self-closing JSX. Could still be confused with
			// `<Foo/>` in non-JSX? No, that's only JSX.
			return true
		}
		if c == '>' {
			// Could be `<number>(...)` type assertion. Look at the
			// character before `<`: type assertions are typically
			// preceded by `=`, `(`, `,`, `[`, `:`, or `return`.
			j := i - 1
			for j >= 0 && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n') {
				j--
			}
			if j >= 0 {
				switch s[j] {
				case '=', '(', ',', '[', ':', '?', '&', '|', '+', '-', '*', '!':
					continue
				}
				// `return <T>x` would also be a type assertion. Detect
				// by reading back the previous word.
				if isJsxNameCont(s[j]) {
					ws := j
					for ws >= 0 && isJsxNameCont(s[ws]) {
						ws--
					}
					word := s[ws+1 : j+1]
					if word == "return" || word == "yield" || word == "throw" || word == "await" {
						continue
					}
				}
			}
			return true
		}
		if c == ' ' || c == '\t' || c == '\n' {
			// `<Foo attr={...}>` — attribute next. Skip a `/* ... */`
			// comment if present, then accept identifier, `{`, or
			// self-closing `/>`. Bare `/` followed by `*` is a comment,
			// not a self-closing tag.
			k := skipJsxWhitespaceAndComments(s, end)
			if k >= len(s) {
				continue
			}
			if isJsxNameStart(s[k]) || s[k] == '{' {
				return true
			}
			if s[k] == '/' && k+1 < len(s) && s[k+1] == '>' {
				return true
			}
		}
	}
	return false
}

// skipJsxWhitespaceAndComments advances past any combination of
// whitespace and `/* ... */` block comments starting at i, returning
// the index of the next significant character.
func skipJsxWhitespaceAndComments(s string, i int) int {
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			j := i + 2
			for j+1 < len(s) && !(s[j] == '*' && s[j+1] == '/') {
				j++
			}
			i = j + 2
			continue
		}
		break
	}
	return i
}

func isJsxNameStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isJsxNameCont(c byte) bool {
	return isJsxNameStart(c) || (c >= '0' && c <= '9') || c == '-' || c == '.'
}

// fixtureTsconfigTemplate generates the per-case tsconfig. Variant
// options (noUncheckedIndexedAccess, exactOptionalPropertyTypes, jsx)
// flip on based on markers in the case source.
func fixtureTsconfigTemplate(noUnchecked, exactOptional, jsx bool, fileExt string) string {
	extraOpts := ""
	if noUnchecked {
		extraOpts += `, "noUncheckedIndexedAccess": true`
	}
	if exactOptional {
		extraOpts += `, "exactOptionalPropertyTypes": true`
	}
	if jsx {
		extraOpts += `, "jsx": "preserve"`
	}
	return fmt.Sprintf(`{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true%s
  },
  "include": ["case.%s", "var-declaration.ts"]
}`, extraOpts, fileExt)
}

func TestNoUnnecessaryTypeAssertion_TypescriptEslintCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixturePath, _ := filepath.Abs("../../../testdata/typescript-eslint/no-unnecessary-type-assertion.test.ts")
	cases, err := tselintcompat.Load(fixturePath, "no-unnecessary-type-assertion")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var passed, failed int
	for _, c := range cases {
		actual, runErr := runCase(t, c, optsFromCase(c))
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

func optsFromCase(c tselintcompat.Case) nounnecessarytypeassertion.Options {
	opts := nounnecessarytypeassertion.DefaultOptions()
	if c.Options == nil {
		return opts
	}
	if raw, ok := c.Options["typesToIgnore"].([]any); ok {
		for _, e := range raw {
			if s, ok := e.(string); ok {
				opts.TypesToIgnore = append(opts.TypesToIgnore, s)
			}
		}
	}
	if v, ok := c.Options["checkLiteralConstAssertions"].(bool); ok {
		opts.CheckLiteralConstAssertions = v
	}
	return opts
}

func runCase(t *testing.T, c tselintcompat.Case, opts nounnecessarytypeassertion.Options) (int, error) {
	t.Helper()
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	noUnchecked := strings.Contains(c.LanguageOptionsText, "noUncheckedIndexedAccess") ||
		strings.Contains(c.LanguageOptionsText, "optionsWithOnUncheckedIndexedAccess")
	exactOptional := strings.Contains(c.LanguageOptionsText, "exactOptionalPropertyTypes") ||
		strings.Contains(c.LanguageOptionsText, "optionsWithExactOptionalPropertyTypes")
	// JSX cases declare a custom `namespace JSX` or use `<Component/>` /
	// `</Component>` self-closing patterns. `<number /* a */>(3+5)` and
	// similar TypeScript type-assertion syntax happen to contain `/>`
	// substrings — match more precisely on JSX-tag shapes (identifier
	// or self-closing right after `<`).
	useJsx := strings.Contains(c.Code, "JSX.IntrinsicElements") ||
		hasJsxTag(c.Code)
	fileExt := "ts"
	if useJsx {
		fileExt = "tsx"
	}
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(fixtureTsconfigTemplate(noUnchecked, exactOptional, useJsx, fileExt)), 0o644)
	os.WriteFile(filepath.Join(dir, "case."+fileExt), []byte(c.Code), 0o644)
	os.WriteFile(filepath.Join(dir, "var-declaration.ts"), []byte("var varDeclarationFromFixture = 1;\n"), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		return 0, err
	}
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nounnecessarytypeassertion.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-unnecessary-type-assertion": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-unnecessary-type-assertion" {
			count++
		}
	}
	return count, nil
}
