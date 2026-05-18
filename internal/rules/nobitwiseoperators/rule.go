// Package nobitwiseoperators implements no-bitwise-operators: in
// most JavaScript code a `&` or `|` is a `&&` typo, since bitwise
// math on doubles is rarely intentional. Banning them surfaces the
// rare legitimate case for explicit review.
package nobitwiseoperators

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-bitwise-operators"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression:        binary,
		wrapperchecker.KindPrefixUnaryExpression:   prefix,
	}
}

var bitwiseOps = map[string]bool{
	"&": true, "|": true, "^": true,
	"<<": true, ">>": true, ">>>": true,
	"&=": true, "|=": true, "^=": true,
	"<<=": true, ">>=": true, ">>>=": true,
}

func binary(ctx *engine.Context, n *wrapperchecker.Node) {
	op := operatorToken(n)
	if bitwiseOps[op] {
		ctx.Report(n, "bitwise operator "+op+" — probably a typo for the boolean equivalent")
	}
}

func prefix(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() == "~" {
		ctx.Report(n, "bitwise NOT (~) — rarely intentional, prefer explicit math")
	}
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
