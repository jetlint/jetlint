// Package noassigninexpressions implements no-assign-in-expressions:
// `if (x = 1)` is almost always a `==` typo. The rule asks for the
// assignment to live on its own statement so the side effect is
// obvious.
package noassigninexpressions

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-assign-in-expressions"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

var assignOps = map[string]bool{
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true,
	"**=": true, "<<=": true, ">>=": true, ">>>=": true,
	"&=": true, "|=": true, "^=": true,
	"&&=": true, "||=": true, "??=": true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := operatorToken(n)
	if !assignOps[op] {
		return
	}
	if isAllowedContext(n) {
		return
	}
	ctx.Report(n, "assignment inside an expression hides a side effect — split it out")
}

// isAllowedContext walks up from the assignment, tunneling through
// parens and `,` expressions, until it finds a context that justifies
// using the assignment's value.
func isAllowedContext(n *wrapperchecker.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindExpressionStatement,
			wrapperchecker.KindForStatement, wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement,
			wrapperchecker.KindVariableDeclaration,
			wrapperchecker.KindReturnStatement,
			wrapperchecker.KindArrowFunction:
			return true
		case wrapperchecker.KindParenthesizedExpression:
			continue
		case wrapperchecker.KindBinaryExpression:
			op := operatorToken(p)
			if op == "," || assignOps[op] {
				continue
			}
			return false
		default:
			return false
		}
	}
	return false
}

func operatorToken(n *wrapperchecker.Node) string {
	var second *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 1 {
			second = c
			return true
		}
		idx++
		return false
	})
	if second == nil {
		return ""
	}
	return second.SourceText()
}
