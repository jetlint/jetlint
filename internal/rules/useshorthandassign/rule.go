// Package useshorthandassign implements use-shorthand-assign:
// `x = x + 1` is `x += 1`. Same for the other binary operators.
package useshorthandassign

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-shorthand-assign"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

var shortenable = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true, "**": true,
	"&": true, "|": true, "^": true, "<<": true, ">>": true, ">>>": true,
	"&&": true, "||": true, "??": true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if operatorToken(n) != "=" {
		return
	}
	target := leftOperand(n)
	rhs := rightOperand(n)
	if target == nil || rhs == nil {
		return
	}
	if target.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if rhs.Kind() != wrapperchecker.KindBinaryExpression {
		return
	}
	op := operatorToken(rhs)
	if !shortenable[op] {
		return
	}
	left := leftOperand(rhs)
	if left == nil || left.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if left.SourceText() != target.SourceText() {
		return
	}
	ctx.Report(n, "use "+op+"= shorthand")
}

func leftOperand(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return first
}

func rightOperand(n *wrapperchecker.Node) *wrapperchecker.Node {
	var third *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 2 {
			third = c
			return true
		}
		idx++
		return false
	})
	return third
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
