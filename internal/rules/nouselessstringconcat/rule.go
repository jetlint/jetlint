// Package nouselessstringconcat implements no-useless-string-concat:
// concatenating two string literals (`"a" + "b"`) can be written as
// one literal.
package nouselessstringconcat

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-string-concat"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.BinaryOperatorKind() != wrapperchecker.KindPlusToken {
		return
	}
	left := stripParens(n.BinaryLeft())
	right := stripParens(n.BinaryRight())
	if !isStringish(left) || !isStringish(right) {
		return
	}
	// Skip if the literals are split across lines (intentional formatting).
	src := n.SourceText()
	// Skip stylistic multi-line concatenations.
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			return
		}
	}
	ctx.Report(n, "two string literals concatenated — combine into one")
}

func isStringish(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return true
	}
	return false
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		n = inner
	}
	return n
}
