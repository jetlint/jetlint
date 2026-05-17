// Package noextrabooleancast implements no-extra-boolean-cast: in
// places where a value is already coerced to boolean (if, while, do-while,
// for condition, ternary test, `!`, `&&` short-circuit chains), an
// extra `Boolean(...)` or `!!...` is redundant.
package noextrabooleancast

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-extra-boolean-cast"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           visitTest,
		wrapperchecker.KindWhileStatement:        visitTest,
		wrapperchecker.KindDoStatement:           visitTest,
		wrapperchecker.KindForStatement:          visitForTest,
		wrapperchecker.KindConditionalExpression: visitTest,
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
	// Unwrap parentheses.
	for test != nil && test.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		test.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		test = inner
	}
	if test == nil {
		return
	}
	// `!!x` — double-negation in a boolean context.
	if test.Kind() == wrapperchecker.KindPrefixUnaryExpression && test.PrefixUnaryOperator() == "!" {
		var inner *wrapperchecker.Node
		test.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner != nil && inner.Kind() == wrapperchecker.KindPrefixUnaryExpression && inner.PrefixUnaryOperator() == "!" {
			ctx.Report(test, "redundant `!!` in a boolean context")
			return
		}
	}
	// `Boolean(x)` call.
	if test.Kind() == wrapperchecker.KindCallExpression {
		var callee *wrapperchecker.Node
		test.ForEachChild(func(c *wrapperchecker.Node) bool {
			if callee == nil {
				callee = c
			}
			return false
		})
		if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier && callee.SourceText() == "Boolean" {
			// Make sure exactly one argument (not Boolean(x, y)).
			argCount := 0
			idx := 0
			test.ForEachChild(func(c *wrapperchecker.Node) bool {
				if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
					argCount++
				}
				idx++
				return false
			})
			if argCount == 1 {
				ctx.Report(test, "redundant `Boolean(...)` in a boolean context")
			}
		}
	}
}
