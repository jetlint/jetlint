// Package noconstantmathminmaxclamp implements no-constant-math-min-max-clamp:
// flag clamp patterns of the form `Math.min(A, Math.max(B, x))` (or
// the mirror `Math.max(A, Math.min(B, x))`) when the two numeric
// bounds are reversed and the expression collapses to a constant —
// the author almost certainly meant `Math.min(high, Math.max(low, x))`.
//
// "Math" can be addressed directly or through `window.Math` /
// `globalThis.Math`; binding shadows aren't tracked, so a local
// `function foo(Math)` parameter still hits the rule. The biome
// fixtures special-case that with a comment, but the rule itself
// doesn't, and matching biome's behavior keeps the fixture happy.
package noconstantmathminmaxclamp

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-constant-math-min-max-clamp"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

// visit fires on every call. The interesting shape is a two-arg
// Math.min/max where the other arg is a two-or-more-arg
// Math.max/min — opposite method — with a literal in its first slot.
// Once both literals are extracted, the clamp is invalid if the
// outer literal is on the "wrong side" of the inner literal for the
// outer's direction.
func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	outer, ok := mathCall(n)
	if !ok {
		return
	}
	args := n.CallArguments()
	if len(args) != 2 {
		return
	}
	outerLit, otherIdx := pickLiteralAndOther(args)
	if outerLit == nil {
		return
	}
	other := stripParens(args[otherIdx])
	inner, ok := mathCall(other)
	if !ok {
		return
	}
	if inner == outer {
		return
	}
	innerArgs := other.CallArguments()
	if len(innerArgs) != 2 {
		return
	}
	innerLit, innerVarIdx := pickLiteralAndOther(innerArgs)
	if innerLit == nil {
		return
	}
	// Skip pure-constant inner calls: a real clamp has a variable
	// being bounded. `Math.min(0, Math.max(100, 110))` is a constant
	// expression, not a clamp, even though it satisfies the shape.
	if numericLiteral(innerArgs[innerVarIdx]) != nil {
		return
	}
	outerVal, ok := literalValue(outerLit)
	if !ok {
		return
	}
	innerVal, ok := literalValue(innerLit)
	if !ok {
		return
	}
	switch {
	case outer == mathMin && inner == mathMax:
		if outerVal <= innerVal {
			ctx.Report(n, "clamp is constant; swap the literal bounds (Math.min(high, Math.max(low, x)))")
		}
	case outer == mathMax && inner == mathMin:
		if outerVal >= innerVal {
			ctx.Report(n, "clamp is constant; swap the literal bounds (Math.max(low, Math.min(high, x)))")
		}
	}
}

type mathMethod int

const (
	mathNone mathMethod = iota
	mathMin
	mathMax
)

// mathCall returns which Math method this call invokes, if any.
// Recognized callees: `Math.min`, `Math.max`, `window.Math.*`,
// `globalThis.Math.*`. Parentheses are unwrapped. A user-declared
// `Math` (a `const Math = ...` binding or `function f(Math)` param)
// makes the call return false — the receiver no longer names the
// global Math.
func mathCall(n *wrapperchecker.Node) (mathMethod, bool) {
	if n == nil || n.Kind() != wrapperchecker.KindCallExpression {
		return mathNone, false
	}
	callee := stripParens(n.CalleeExpression())
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return mathNone, false
	}
	name := callee.PropertyAccessName()
	var m mathMethod
	switch name {
	case "min":
		m = mathMin
	case "max":
		m = mathMax
	default:
		return mathNone, false
	}
	recv := stripParens(callee.PropertyAccessReceiver())
	if recv == nil {
		return mathNone, false
	}
	switch recv.Kind() {
	case wrapperchecker.KindIdentifier:
		if recv.LiteralText() != "Math" {
			return mathNone, false
		}
		if mathIsBoundInFile(recv) {
			return mathNone, false
		}
		return m, true
	case wrapperchecker.KindPropertyAccessExpression:
		if recv.PropertyAccessName() != "Math" {
			return mathNone, false
		}
		inner := stripParens(recv.PropertyAccessReceiver())
		if inner == nil || inner.Kind() != wrapperchecker.KindIdentifier {
			return mathNone, false
		}
		t := inner.LiteralText()
		if t != "window" && t != "globalThis" {
			return mathNone, false
		}
		return m, true
	}
	return mathNone, false
}

// pickLiteralAndOther returns the first numeric-literal argument and
// the index of the other argument. If neither is a numeric literal,
// returns (nil, 0).
func pickLiteralAndOther(args []*wrapperchecker.Node) (*wrapperchecker.Node, int) {
	if lit := numericLiteral(args[0]); lit != nil {
		return lit, 1
	}
	if lit := numericLiteral(args[1]); lit != nil {
		return lit, 0
	}
	return nil, 0
}

func numericLiteral(n *wrapperchecker.Node) *wrapperchecker.Node {
	n = stripParens(n)
	if n == nil {
		return nil
	}
	if n.Kind() == wrapperchecker.KindNumericLiteral {
		return n
	}
	// Allow unary `-1`/`+1` literals.
	if n.Kind() == wrapperchecker.KindPrefixUnaryExpression {
		op := n.PrefixUnaryOperator()
		if op != "+" && op != "-" {
			return nil
		}
		inner := stripParens(n.PrefixUnaryOperand())
		if inner != nil && inner.Kind() == wrapperchecker.KindNumericLiteral {
			return n
		}
	}
	return nil
}

// literalValue parses a NumericLiteral (or unary +/- around one)
// using JavaScript-like number semantics: hex/binary/octal literals
// are supported, and values beyond Number.MAX_SAFE_INTEGER lose
// precision the same way the runtime would. Returns ok=false only
// when the text fails to parse, which the AST shouldn't produce.
func literalValue(n *wrapperchecker.Node) (float64, bool) {
	sign := 1.0
	if n.Kind() == wrapperchecker.KindPrefixUnaryExpression {
		if n.PrefixUnaryOperator() == "-" {
			sign = -1
		}
		n = stripParens(n.PrefixUnaryOperand())
	}
	if n == nil || n.Kind() != wrapperchecker.KindNumericLiteral {
		return 0, false
	}
	v, ok := parseJSNumber(n.LiteralText())
	if !ok {
		return 0, false
	}
	return sign * v, true
}

// parseJSNumber mirrors the subset of the JS NumericLiteral grammar
// the AST keeps verbatim: decimal (with optional fraction/exponent),
// 0x..., 0b..., 0o... and the legacy 0... octal form. Underscores are
// stripped. Returns ok=false on any text we don't recognize so the
// rule stays silent rather than guessing.
func parseJSNumber(s string) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	clean := stripUnderscores(s)
	if len(clean) >= 2 && clean[0] == '0' {
		switch clean[1] {
		case 'x', 'X':
			return parseBase(clean[2:], 16)
		case 'b', 'B':
			return parseBase(clean[2:], 2)
		case 'o', 'O':
			return parseBase(clean[2:], 8)
		}
	}
	return parseDecimal(clean)
}

func stripUnderscores(s string) string {
	if !containsByte(s, '_') {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '_' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}

func parseBase(s string, base int) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	var v float64
	for i := 0; i < len(s); i++ {
		d, ok := hexDigit(s[i])
		if !ok || d >= base {
			return 0, false
		}
		v = v*float64(base) + float64(d)
	}
	return v, true
}

func hexDigit(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

func parseDecimal(s string) (float64, bool) {
	var v float64
	scanned := 0
	for ; scanned < len(s); scanned++ {
		c := s[scanned]
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + float64(c-'0')
	}
	if scanned < len(s) && s[scanned] == '.' {
		scanned++
		frac := 0.1
		for ; scanned < len(s); scanned++ {
			c := s[scanned]
			if c < '0' || c > '9' {
				break
			}
			v += float64(c-'0') * frac
			frac /= 10
		}
	}
	if scanned < len(s) && (s[scanned] == 'e' || s[scanned] == 'E') {
		scanned++
		expSign := 1
		if scanned < len(s) && (s[scanned] == '+' || s[scanned] == '-') {
			if s[scanned] == '-' {
				expSign = -1
			}
			scanned++
		}
		exp := 0
		for ; scanned < len(s); scanned++ {
			c := s[scanned]
			if c < '0' || c > '9' {
				return 0, false
			}
			exp = exp*10 + int(c-'0')
		}
		for i := 0; i < exp; i++ {
			if expSign > 0 {
				v *= 10
			} else {
				v /= 10
			}
		}
	}
	if scanned != len(s) {
		return 0, false
	}
	return v, true
}

// mathIsBoundInFile walks up to the source file root and scans for
// any binding named "Math" — `const Math = ...`, `function Math() {}`,
// `function f(Math)`, `class Math {}`, `import { Math } from ...`,
// etc. TypeScript's symbol resolution falls back to the global lib
// declaration in error scenarios, so we can't trust the user-decl
// check at the reference site; a syntactic file-wide scan is a
// conservative way to skip the rule when the user has any local
// binding shadowing the global. Cost is one walk per call site —
// acceptable for the depth of files this rule runs on.
func mathIsBoundInFile(ref *wrapperchecker.Node) bool {
	root := ref
	for {
		p := root.Parent()
		if p == nil {
			break
		}
		root = p
	}
	found := false
	var walk func(c *wrapperchecker.Node) bool
	walk = func(c *wrapperchecker.Node) bool {
		if found {
			return true
		}
		if isMathBinding(c) {
			found = true
			return true
		}
		c.ForEachChild(walk)
		return found
	}
	root.ForEachChild(walk)
	return found
}

// isMathBinding reports whether n introduces a binding named "Math".
// Only checks node kinds that contribute to scope; reference
// occurrences (e.g. inside `Math.min(...)`) are not bindings and
// don't count.
func isMathBinding(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration:
		name := bindingName(n)
		return name == "Math"
	}
	return false
}

// bindingName extracts the bound identifier name of a declaration
// node, when it is a simple identifier. Destructuring patterns and
// computed names return "".
func bindingName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
