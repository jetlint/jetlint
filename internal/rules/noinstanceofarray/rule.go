// Package noinstanceofarray implements the no-instanceof-array rule:
// `x instanceof Array` doesn't work across realms (iframes, vm
// contexts). Prefer `Array.isArray(x)`.
//
// Matches oxlint's unicorn/no-instanceof-array: zero configurable
// options. Reports each binary `instanceof Array` (right operand
// ignoring parentheses).
package noinstanceofarray

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-instanceof-array"

// New constructs a no-instanceof-array rule.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if operatorText(n) != "instanceof" {
		return
	}
	right := stripParens(n.BinaryRight())
	if right == nil || right.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if right.LiteralText() != "Array" {
		return
	}
	ctx.Report(n, "Use `Array.isArray()` instead of `instanceof Array`.")
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// operatorText extracts the binary operator text from the source span
// between the two operands. The wrapper does not export a token-kind
// constant for `instanceof`, so we read it from source.
func operatorText(n *wrapperchecker.Node) string {
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return ""
	}
	src := n.SourceText()
	nodeStart := n.End() - len(src)
	startRel := left.End() - nodeStart
	endRel := right.Pos() - nodeStart
	if startRel < 0 || endRel <= startRel || endRel > len(src) {
		return ""
	}
	return strings.TrimSpace(src[startRel:endRel])
}
