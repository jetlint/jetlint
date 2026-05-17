// Package nomisplacedassertion implements no-misplaced-assertion:
// `expect(...)` or `assert.*` should appear inside a `test`/`it`
// callback, not at the top level or directly inside `describe`.
package nomisplacedassertion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-misplaced-assertion"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isAssertion(n) {
		return
	}
	// Walk up looking for a test/it/Deno.test/etc callback boundary.
	for p := n.Parent(); p != nil; p = p.Parent() {
		if isInsideTestCallback(p) {
			return
		}
	}
	ctx.Report(n, "assertion is not inside a test callback")
}

func isAssertion(n *wrapperchecker.Node) bool {
	callee := firstChild(n)
	if callee == nil {
		return false
	}
	if callee.Kind() == wrapperchecker.KindIdentifier {
		switch callee.SourceText() {
		case "expect", "assertEquals", "assertNotEquals", "assertStrictEquals",
			"assertThrows", "assertRejects":
			return true
		}
		return false
	}
	if callee.Kind() == wrapperchecker.KindPropertyAccessExpression {
		var obj *wrapperchecker.Node
		callee.ForEachChild(func(c *wrapperchecker.Node) bool {
			if obj == nil {
				obj = c
			}
			return false
		})
		if obj != nil && obj.Kind() == wrapperchecker.KindIdentifier && obj.SourceText() == "assert" {
			return true
		}
	}
	return false
}

func isInsideTestCallback(p *wrapperchecker.Node) bool {
	if p.Kind() != wrapperchecker.KindArrowFunction && p.Kind() != wrapperchecker.KindFunctionExpression {
		return false
	}
	// Walk up CallExpressions until we find one whose callee starts with
	// `test`, `it`, `specify`, `Deno.test`, or `waitFor`.
	for gp := p.Parent(); gp != nil; {
		if gp.Kind() != wrapperchecker.KindCallExpression {
			return false
		}
		callee := firstChild(gp)
		if callee == nil {
			return false
		}
		if calleeRootMatchesTest(callee) {
			return true
		}
		gp = gp.Parent()
	}
	return false
}

func calleeRootMatchesTest(callee *wrapperchecker.Node) bool {
	cur := callee
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindIdentifier:
			switch cur.SourceText() {
			case "test", "it", "specify", "waitFor":
				return true
			}
			return false
		case wrapperchecker.KindPropertyAccessExpression:
			var obj, prop *wrapperchecker.Node
			cur.ForEachChild(func(c *wrapperchecker.Node) bool {
				if obj == nil {
					obj = c
				} else if prop == nil {
					prop = c
				}
				return false
			})
			// Special-case Deno.test.
			if obj != nil && obj.Kind() == wrapperchecker.KindIdentifier && obj.SourceText() == "Deno" &&
				prop != nil && prop.SourceText() == "test" {
				return true
			}
			cur = obj
		case wrapperchecker.KindCallExpression, wrapperchecker.KindTaggedTemplateExpression:
			var c *wrapperchecker.Node
			cur.ForEachChild(func(x *wrapperchecker.Node) bool {
				if c == nil {
					c = x
				}
				return false
			})
			cur = c
		default:
			return false
		}
	}
	return false
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
