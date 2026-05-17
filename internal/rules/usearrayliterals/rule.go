// Package usearrayliterals implements use-array-literals: `Array()` /
// `new Array(0, 1, 2)` should be `[]` / `[0, 1, 2]`. `new Array(N)`
// for a single numeric argument is preallocation and left alone.
package usearrayliterals

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-array-literals"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
		wrapperchecker.KindNewExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee, hasTypeArgs := calleeOf(n)
	if callee == nil {
		return
	}
	if callee.Kind() != wrapperchecker.KindIdentifier || callee.SourceText() != "Array" {
		return
	}
	if isShadowed(n, "Array") {
		return
	}
	_ = hasTypeArgs
	// Find arguments.
	args := callArgs(n)
	switch len(args) {
	case 0:
		// `Array()` or `new Array()` → `[]`.
	case 1:
		// Single arg: only flag if it's not a number (or is a spread).
		a := args[0]
		if a.Kind() == wrapperchecker.KindSpreadElement {
			// `Array(...x)` — biome flags.
		} else if a.Kind() == wrapperchecker.KindNumericLiteral {
			return
		} else {
			return // unknown, keep
		}
	default:
		// Multiple args — flag.
	}
	ctx.Report(n, "use an array literal instead of `Array(...)`")
}

func calleeOf(n *wrapperchecker.Node) (*wrapperchecker.Node, bool) {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	// Scan source for type args / paren placement.
	src := n.SourceText()
	parenIdx := strings.IndexByte(src, '(')
	angleIdx := strings.IndexByte(src, '<')
	hasTypeArgs := angleIdx >= 0 && (parenIdx < 0 || angleIdx < parenIdx)
	return first, hasTypeArgs
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindConstructor:
			if functionHasParam(p, name) {
				return true
			}
		}
	}
	return false
}

func functionHasParam(fn *wrapperchecker.Node, name string) bool {
	found := false
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			var pn *wrapperchecker.Node
			c.ForEachChild(func(p *wrapperchecker.Node) bool {
				if pn == nil {
					pn = p
				}
				return false
			})
			if pn != nil && pn.Kind() == wrapperchecker.KindIdentifier && pn.SourceText() == name {
				found = true
			}
		}
		return false
	})
	return found
}

func callArgs(n *wrapperchecker.Node) []*wrapperchecker.Node {
	// Find the position of `(` in the source so we can filter children
	// that are real arguments (positioned after the paren) from type
	// arguments (positioned before it).
	src := n.SourceText()
	parenIdx := strings.IndexByte(src, '(')
	if parenIdx < 0 {
		return nil
	}
	parenAbs := n.Pos() + parenIdx
	var args []*wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Pos() > parenAbs {
			args = append(args, c)
		}
		idx++
		return false
	})
	return args
}
