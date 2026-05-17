// Package useflatmap implements use-flat-map: `arr.map(f).flat()`
// can be replaced by `arr.flatMap(f)`.
package useflatmap

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-flat-map"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	obj, name := propParts(callee)
	if name != "flat" {
		return
	}
	// .flat() may have 0 or 1 args. 0 = depth 1. 1 must be literal `1`.
	args := callArgs(n)
	switch len(args) {
	case 0:
		// depth defaults to 1, OK
	case 1:
		// must be the number literal 1
		if args[0].Kind() != wrapperchecker.KindNumericLiteral || args[0].SourceText() != "1" {
			return
		}
	default:
		return
	}
	// `obj` must itself be a call to `.map(...)`.
	if obj == nil || obj.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	innerCallee := firstChild(obj)
	if innerCallee == nil || innerCallee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	_, innerName := propParts(innerCallee)
	if innerName != "map" {
		return
	}
	// `.map(...)` must have at most 1 arg (the callback).
	innerArgs := callArgs(obj)
	if len(innerArgs) > 1 {
		return
	}
	ctx.Report(n, ".map(f).flat() can be .flatMap(f)")
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

func callArgs(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var args []*wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
			args = append(args, c)
		}
		idx++
		return false
	})
	return args
}
