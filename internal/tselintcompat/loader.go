// Package tselintcompat extracts test cases from typescript-eslint's
// RuleTester test files so we can run them through our rules and track
// behavioral compatibility.
//
// A typescript-eslint test file calls ruleTester.run() with a config
// object of shape { valid: [...], invalid: [...] }. Each element is
// either a bare string/template literal (the source code under test) or
// an object literal { code, options?, errors? }. We don't execute the
// JS — we parse it as a TS source file and walk the AST to extract the
// cases as data.
package tselintcompat

import (
	"fmt"
	"os"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
)

// Case is one extracted RuleTester case.
type Case struct {
	// Code is the source text the rule is being run against.
	Code string
	// Valid is true for entries from `valid:`, false for `invalid:`.
	Valid bool
	// ExpectedErrorCount is the length of the `errors:` array for
	// invalid cases. Zero for valid cases.
	ExpectedErrorCount int
	// HasOptions is true when the case has a non-empty `options:` array.
	HasOptions bool
	// Options is the parsed contents of `options[0]` for the case (the
	// single-element option object typescript-eslint passes to the
	// rule). Non-nil only when the case has options. Values are the
	// AST-extracted Go representations: bools as bool, arrays as
	// []any, objects as map[string]any, string literals as string.
	Options map[string]any
	// AllOptions is the parsed contents of the `options:` array in
	// declaration order, so rules whose option shape spans multiple
	// elements (e.g. prefer-destructuring's `[ { object: true }, {
	// enforceForRenamedProperties: true } ]`) can still see every
	// piece. Each element is an extracted literal (object →
	// map[string]any, primitive → bool/string/etc., nil for shapes the
	// loader can't model).
	AllOptions []any
	// SourceIndex is the 0-based position of the case within the
	// containing array (valid or invalid). Useful for stable test names.
	SourceIndex int
	// Unstrict is true when the case opts into the upstream `unstrict`
	// fixture tsconfig via
	// `languageOptions.parserOptions.tsconfigRootDir: path.join(rootDir, 'unstrict')`.
	// The harness uses this to compile the case with `strict: false` so
	// rules whose behavior depends on `strictNullChecks` (e.g.
	// prefer-nullish-coalescing's `noStrictNullCheck` diagnostic) can be
	// exercised.
	Unstrict bool
}

// Load parses the given typescript-eslint test file at path and returns
// every case from the first ruleTester.run() call matching ruleName.
func Load(path, ruleName string) ([]Case, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	sf := wrapperchecker.ParseFile(path, string(src))
	if sf == nil {
		return nil, fmt.Errorf("parse fixture: nil source file")
	}

	var cases []Case
	var found bool
	sf.Walk(func(n *wrapperchecker.Node) bool {
		if found {
			return false
		}
		if n.Kind() != wrapperchecker.KindCallExpression {
			return true
		}
		if !isRuleTesterRun(n, ruleName) {
			return true
		}
		args := n.CallArguments()
		if len(args) < 3 {
			return true
		}
		config := args[2]
		if config.Kind() != wrapperchecker.KindObjectLiteralExpression {
			return true
		}
		for _, prop := range config.ObjectProperties() {
			name := prop.PropertyName()
			if name != "valid" && name != "invalid" {
				continue
			}
			arr := prop.PropertyInitializer()
			if arr == nil || arr.Kind() != wrapperchecker.KindArrayLiteralExpression {
				continue
			}
			cases = append(cases, extractCases(arr, name == "valid")...)
		}
		found = true
		return false
	})

	if !found {
		return nil, fmt.Errorf("no ruleTester.run(%q, ...) call found in %s", ruleName, path)
	}
	return cases, nil
}

// isRuleTesterRun reports whether call is `<anything>.run('<ruleName>', ...)`.
// The receiver name varies (ruleTester, useProgramRuleTester, etc.) so
// we only match by method name and first-argument string.
func isRuleTesterRun(call *wrapperchecker.Node, ruleName string) bool {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if callee.PropertyAccessName() != "run" {
		return false
	}
	args := call.CallArguments()
	if len(args) < 1 {
		return false
	}
	first := args[0]
	switch first.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
	default:
		return false
	}
	return first.LiteralText() == ruleName
}

func extractCases(arr *wrapperchecker.Node, valid bool) []Case {
	var out []Case
	for i, elem := range arr.ArrayElements() {
		c, ok := caseFromElement(elem, valid)
		if !ok {
			continue
		}
		c.SourceIndex = i
		out = append(out, c)
	}
	return out
}

// caseFromElement builds a Case from a single array element. Returns
// ok=false for shapes we can't handle (e.g. spread elements, calls
// returning a generated case — typescript-eslint does this in some
// rules).
func caseFromElement(elem *wrapperchecker.Node, valid bool) (Case, bool) {
	switch elem.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return Case{Code: elem.LiteralText(), Valid: valid}, true
	case wrapperchecker.KindObjectLiteralExpression:
		c := Case{Valid: valid}
		gotCode := false
		for _, prop := range elem.ObjectProperties() {
			switch prop.PropertyName() {
			case "code":
				init := prop.PropertyInitializer()
				if init == nil {
					return Case{}, false
				}
				switch init.Kind() {
				case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
					c.Code = init.LiteralText()
					gotCode = true
				default:
					return Case{}, false
				}
			case "errors":
				init := prop.PropertyInitializer()
				if init != nil && init.Kind() == wrapperchecker.KindArrayLiteralExpression {
					c.ExpectedErrorCount = len(init.ArrayElements())
				}
			case "options":
				init := prop.PropertyInitializer()
				if init != nil && init.Kind() == wrapperchecker.KindArrayLiteralExpression {
					elems := init.ArrayElements()
					if len(elems) > 0 {
						c.HasOptions = true
						c.AllOptions = make([]any, 0, len(elems))
						for _, e := range elems {
							c.AllOptions = append(c.AllOptions, literalValue(e))
						}
						if first := elems[0]; first.Kind() == wrapperchecker.KindObjectLiteralExpression {
							if obj, ok := literalValue(first).(map[string]any); ok {
								c.Options = obj
							}
						}
					}
				}
			case "languageOptions":
				if init := prop.PropertyInitializer(); init != nil {
					c.Unstrict = referencesUnstrictFixture(init)
				}
			}
		}
		if !gotCode {
			return Case{}, false
		}
		return c, true
	default:
		return Case{}, false
	}
}

// referencesUnstrictFixture reports whether n contains a string literal
// `'unstrict'` anywhere in its subtree. Upstream fixtures opt into the
// non-strict tsconfig via
// `languageOptions: { parserOptions: { tsconfigRootDir: path.join(rootDir, 'unstrict') } }`,
// so any `'unstrict'` reachable through the languageOptions object
// identifies a case that needs the unstrict tsconfig in our harness.
func referencesUnstrictFixture(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindStringLiteral ||
		n.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		if n.LiteralText() == "unstrict" {
			return true
		}
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if referencesUnstrictFixture(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

// literalValue extracts a Go-typed representation of a JS literal AST
// node. Supports the shapes that appear in typescript-eslint test
// fixtures: object/array literals, string and template literals,
// numeric literals, and the keyword identifiers `true`/`false`/`null`.
// Returns nil for shapes we don't model (function expressions, calls,
// etc.) — callers should treat nil as "unknown" and ignore.
func literalValue(n *wrapperchecker.Node) any {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText()
	case wrapperchecker.KindNumericLiteral:
		return n.LiteralText()
	case wrapperchecker.KindTrueKeyword:
		return true
	case wrapperchecker.KindFalseKeyword:
		return false
	case wrapperchecker.KindNullKeyword:
		return nil
	case wrapperchecker.KindIdentifier:
		if n.LiteralText() == "undefined" {
			return nil
		}
		return n.LiteralText()
	case wrapperchecker.KindArrayLiteralExpression:
		elems := n.ArrayElements()
		out := make([]any, 0, len(elems))
		for _, e := range elems {
			out = append(out, literalValue(e))
		}
		return out
	case wrapperchecker.KindObjectLiteralExpression:
		out := map[string]any{}
		for _, p := range n.ObjectProperties() {
			if p.Kind() != wrapperchecker.KindPropertyAssignment {
				continue
			}
			out[p.PropertyName()] = literalValue(p.PropertyInitializer())
		}
		return out
	case wrapperchecker.KindRegularExpressionLiteral:
		return regexSource(n.LiteralText())
	case wrapperchecker.KindPropertyAccessExpression:
		// Support `/regex/.source` — extract the regex literal's source.
		if name := n.PropertyAccessName(); name == "source" {
			recv := n.PropertyAccessReceiver()
			if recv != nil && recv.Kind() == wrapperchecker.KindRegularExpressionLiteral {
				return regexSource(recv.LiteralText())
			}
		}
	}
	return nil
}

// regexSource strips the surrounding `/` delimiters and trailing flags
// from a regex literal's text, returning just the pattern source.
func regexSource(lit string) string {
	if len(lit) < 2 || lit[0] != '/' {
		return lit
	}
	// Strip trailing flags by finding the last `/`.
	last := -1
	for i := len(lit) - 1; i > 0; i-- {
		if lit[i] == '/' {
			last = i
			break
		}
	}
	if last <= 0 {
		return lit
	}
	return lit[1:last]
}
