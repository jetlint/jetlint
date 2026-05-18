// Package noduplicatetesthooks implements no-duplicate-test-hooks:
// declaring `beforeEach` (or `beforeAll`, `afterEach`, `afterAll`)
// twice in the same describe block accumulates effects in a way that's
// almost never intended.
package noduplicatetesthooks

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-duplicate-test-hooks"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

var hookNames = map[string]bool{
	"beforeEach": true,
	"beforeAll":  true,
	"afterEach":  true,
	"afterAll":  true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isDescribeCall(n) {
		return
	}
	body := describeBody(n)
	if body == nil {
		return
	}
	counts := map[string]int{}
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindExpressionStatement {
			return false
		}
		var inner *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if inner == nil {
				inner = d
			}
			return false
		})
		if inner == nil || inner.Kind() != wrapperchecker.KindCallExpression {
			return false
		}
		callee := firstChild(inner)
		if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		name := callee.SourceText()
		if hookNames[name] {
			counts[name]++
			if counts[name] > 1 {
				ctx.Report(inner, "duplicate `"+name+"` hook in this describe block")
			}
		}
		return false
	})
}

func isDescribeCall(n *wrapperchecker.Node) bool {
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	name := callee.SourceText()
	return name == "describe" || name == "suite" || name == "context"
}

func describeBody(call *wrapperchecker.Node) *wrapperchecker.Node {
	var args []*wrapperchecker.Node
	idx := 0
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 {
			args = append(args, c)
		}
		idx++
		return false
	})
	if len(args) < 2 {
		return nil
	}
	fn := args[1]
	if fn.Kind() != wrapperchecker.KindArrowFunction && fn.Kind() != wrapperchecker.KindFunctionExpression {
		return nil
	}
	var body *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	return body
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
