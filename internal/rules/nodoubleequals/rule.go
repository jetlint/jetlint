// Package nodoubleequals implements no-double-equals: `==` and `!=`
// do implicit type coercion, which surprises everyone. Use `===` /
// `!==` and be explicit about coercion when you want it.
package nodoubleequals

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-double-equals"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := operatorToken(n)
	if op != "==" && op != "!=" {
		return
	}
	// `x == null` is an idiomatic check for nullish — allow it.
	if isNullOperand(leftOperand(n)) || isNullOperand(rightOperand(n)) {
		return
	}
	ctx.Report(n, "use === / !== — `"+op+"` coerces types implicitly")
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

func isNullOperand(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	return n.Kind() == wrapperchecker.KindNullKeyword
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
