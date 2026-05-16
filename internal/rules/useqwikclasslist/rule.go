// Package useqwikclasslist implements use-qwik-classlist: Qwik's
// JSX `class` attribute understands arrays, objects, and strings
// natively (Qwik calls this "classlist" syntax), so a call to the
// classnames / clsx library is unnecessary indirection that costs
// a runtime dependency and trips Qwik's serializer. The rule flags
// `class={classnames(...)}` (and clsx variants) so the call can be
// replaced with the native array/object form.
package useqwikclasslist

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-qwik-classlist"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxAttribute: visit,
	}
}

func visit(ctx *engine.Context, attr *wrapperchecker.Node) {
	if !attributeNameIs(attr, "class") {
		return
	}
	expr := attributeExpression(attr)
	if expr == nil {
		return
	}
	if !isClassnamesCall(expr) {
		return
	}
	ctx.Report(expr, "replace the classnames call with Qwik's native class array/object syntax")
}

// attributeNameIs checks the JsxAttribute's name child against the
// expected attribute name (e.g., "class").
func attributeNameIs(attr *wrapperchecker.Node, name string) bool {
	var match bool
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
			match = true
			return true
		}
		return false
	})
	return match
}

// attributeExpression returns the inner expression of a JSX
// attribute value that is wrapped in `{...}`. Returns nil when the
// attribute has no value or its value isn't a brace expression.
func attributeExpression(attr *wrapperchecker.Node) *wrapperchecker.Node {
	var jsxExpr *wrapperchecker.Node
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindJsxExpression {
			jsxExpr = c
			return true
		}
		return false
	})
	if jsxExpr == nil {
		return nil
	}
	var inner *wrapperchecker.Node
	jsxExpr.ForEachChild(func(c *wrapperchecker.Node) bool {
		inner = c
		return true
	})
	return inner
}

// isClassnamesCall reports whether the expression is a call to one
// of the well-known class-string-builder libraries (`classnames`,
// `clsx`, `cn`). These are the libraries Qwik documentation
// explicitly suggests replacing with classlist.
func isClassnamesCall(n *wrapperchecker.Node) bool {
	if n.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	switch callee.LiteralText() {
	case "classnames", "clsx", "cn":
		return true
	}
	return false
}
