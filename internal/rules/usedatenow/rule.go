// Package usedatenow implements use-date-now: `Date.now()` allocates
// no object and reads obvious. `new Date().getTime()` (or `+ new Date()`)
// does the same thing the long way.
package usedatenow

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-date-now"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:        visitCall,
		wrapperchecker.KindPrefixUnaryExpression: visitUnary,
	}
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	// `new Date().getTime()` / `.valueOf()`
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	obj, name := propParts(callee)
	if name != "getTime" && name != "valueOf" {
		return
	}
	if obj == nil {
		return
	}
	if obj.Kind() == wrapperchecker.KindParenthesizedExpression {
		obj = firstChild(obj)
	}
	if obj == nil || obj.Kind() != wrapperchecker.KindNewExpression {
		return
	}
	newCallee := firstChild(obj)
	if newCallee == nil || newCallee.SourceText() != "Date" {
		return
	}
	// `new Date(...)` must have zero args.
	if callArgCount(obj) != 0 {
		return
	}
	// `.getTime(...)` must have zero args.
	if callArgCount(n) != 0 {
		return
	}
	ctx.Report(n, "`new Date()."+name+"()` allocates an object you don't need — use `Date.now()`")
}

func visitUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "+" {
		return
	}
	op := n.PrefixUnaryOperand()
	if op == nil {
		return
	}
	if op.Kind() == wrapperchecker.KindParenthesizedExpression {
		op = firstChild(op)
	}
	if op == nil || op.Kind() != wrapperchecker.KindNewExpression {
		return
	}
	newCallee := firstChild(op)
	if newCallee == nil || strings.TrimSpace(newCallee.SourceText()) != "Date" {
		return
	}
	if callArgCount(op) != 0 {
		return
	}
	ctx.Report(n, "`+new Date()` allocates an object you don't need — use `Date.now()`")
}

func callArgCount(n *wrapperchecker.Node) int {
	count := 0
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
			count++
		}
		idx++
		return false
	})
	return count
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}

func propParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return nil, ""
	}
	return first, second.SourceText()
}
