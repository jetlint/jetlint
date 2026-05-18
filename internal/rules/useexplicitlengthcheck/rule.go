// Package useexplicitlengthcheck implements use-explicit-length-check:
// `if (foo.length)` is ambiguous about whether you mean `> 0` or
// "truthy", so compare explicitly.
package useexplicitlengthcheck

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-explicit-length-check"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           visitTest,
		wrapperchecker.KindWhileStatement:        visitTest,
		wrapperchecker.KindDoStatement:           visitTest,
		wrapperchecker.KindConditionalExpression: visitTest,
		wrapperchecker.KindForStatement:          visitForTest,
		wrapperchecker.KindPrefixUnaryExpression: visitUnary,
	}
}

func visitTest(ctx *engine.Context, n *wrapperchecker.Node) {
	var test *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if test == nil {
			test = c
		}
		return false
	})
	check(ctx, test)
}

func visitForTest(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n.ForStatementCondition())
}

func visitUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	var inner *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if inner == nil {
			inner = c
		}
		return false
	})
	check(ctx, inner)
}

func check(ctx *engine.Context, test *wrapperchecker.Node) {
	if test == nil {
		return
	}
	// Walk through `!`, parentheses, `Boolean(...)`.
	for test != nil {
		switch test.Kind() {
		case wrapperchecker.KindParenthesizedExpression, wrapperchecker.KindPrefixUnaryExpression:
			var inner *wrapperchecker.Node
			test.ForEachChild(func(c *wrapperchecker.Node) bool {
				if inner == nil {
					inner = c
				}
				return false
			})
			test = inner
			continue
		}
		break
	}
	if test == nil {
		return
	}
	if isLengthAccess(test) {
		ctx.Report(test, "compare `.length` explicitly to a number")
	}
}

func isLengthAccess(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
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
		return false
	}
	return second.SourceText() == "length"
}
