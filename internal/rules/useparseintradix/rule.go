// Package useparseintradix implements use-parse-int-radix
// (eslint's radix rule, biome's use-parse-int-radix): require an
// explicit radix argument on every `parseInt(...)` and
// `Number.parseInt(...)` call. Without one, parseInt's behavior is
// version-dependent — historically it could infer octal for `"08"`
// or hex for `"0xF"`, and forgetting the argument is a frequent
// source of "wrong answer" bugs.
//
// The radix must be a numeric literal in the closed range [2, 36].
// `undefined` and BigInt/string literals are rejected. Non-literal
// expressions (a variable, function call, etc.) are accepted —
// the analysis can't see their runtime value and false positives
// would be common.
package useparseintradix

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-parse-int-radix"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isParseIntCallee(n.CalleeExpression()) {
		return
	}
	args := n.CallArguments()
	if len(args) < 2 {
		ctx.Report(n, "parseInt() called without a radix; pass 10 for decimal")
		return
	}
	if radixIssue(args[1]) {
		ctx.Report(n, "parseInt() radix must be an integer literal in [2, 36]")
	}
}

// isParseIntCallee reports whether the callee is `parseInt` or
// `Number.parseInt` (with `Number` resolving by name; not scope-
// aware). Parentheses are unwrapped. `globalThis.parseInt`,
// `window.parseInt`, and shadowed local `parseInt` are out of
// scope — biome's fixtures don't exercise them.
func isParseIntCallee(callee *wrapperchecker.Node) bool {
	callee = stripParens(callee)
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return callee.LiteralText() == "parseInt"
	case wrapperchecker.KindPropertyAccessExpression:
		if callee.PropertyAccessName() != "parseInt" {
			return false
		}
		recv := stripParens(callee.PropertyAccessReceiver())
		return recv != nil &&
			recv.Kind() == wrapperchecker.KindIdentifier &&
			recv.LiteralText() == "Number"
	}
	return false
}

// radixIssue returns true when the supplied second argument is a
// statically detectable bad radix. Returns false for non-literal
// expressions (defensive: the actual runtime value isn't visible
// here).
func radixIssue(arg *wrapperchecker.Node) bool {
	a := stripParens(arg)
	if a == nil {
		return false
	}
	switch a.Kind() {
	case wrapperchecker.KindIdentifier:
		return a.LiteralText() == "undefined"
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindBigIntLiteral:
		return true
	case wrapperchecker.KindNumericLiteral:
		v, ok := parseIntegerLiteral(a.LiteralText())
		if !ok {
			return true
		}
		return v < 2 || v > 36
	case wrapperchecker.KindPrefixUnaryExpression:
		op := a.PrefixUnaryOperator()
		if op != "+" && op != "-" {
			return false
		}
		operand := stripParens(a.PrefixUnaryOperand())
		if operand == nil || operand.Kind() != wrapperchecker.KindNumericLiteral {
			return false
		}
		v, ok := parseIntegerLiteral(operand.LiteralText())
		if !ok {
			return true
		}
		if op == "-" {
			v = -v
		}
		return v < 2 || v > 36
	}
	return false
}

// parseIntegerLiteral accepts the integer subset of the JS
// NumericLiteral grammar: decimal, 0x, 0b, 0o, and numeric
// separators. Floats and exponents are rejected (a radix must be
// an integer).
func parseIntegerLiteral(s string) (int, bool) {
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
	v := 0
	for i := 0; i < len(clean); i++ {
		c := clean[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int(c-'0')
	}
	return v, true
}

func parseBase(s string, base int) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	v := 0
	for i := 0; i < len(s); i++ {
		d, ok := hexDigit(s[i])
		if !ok || d >= base {
			return 0, false
		}
		v = v*base + d
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

func stripUnderscores(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			out := make([]byte, 0, len(s))
			for j := 0; j < len(s); j++ {
				if s[j] != '_' {
					out = append(out, s[j])
				}
			}
			return string(out)
		}
	}
	return s
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
