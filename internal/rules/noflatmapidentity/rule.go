// Package noflatmapidentity implements no-flat-map-identity: `arr.flatMap(x => x)`
// is just `arr.flat()`. Calling `flatMap` with the identity callback is
// a leftover from a refactor.
package noflatmapidentity

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-flat-map-identity"

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
	_, name := propParts(callee)
	if name != "flatMap" {
		return
	}
	// First arg must be an identity arrow `x => x` or `function (x) { return x }`.
	var firstArg *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && firstArg == nil && c.Kind() != wrapperchecker.KindTypeReference {
			firstArg = c
		}
		idx++
		return false
	})
	if firstArg == nil {
		return
	}
	if isIdentityArrow(firstArg) {
		ctx.Report(n, ".flatMap(x => x) is just .flat()")
	}
}

func isIdentityArrow(n *wrapperchecker.Node) bool {
	if n.Kind() != wrapperchecker.KindArrowFunction {
		return false
	}
	// Find single parameter name and body identifier.
	var paramName string
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			var id *wrapperchecker.Node
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if id == nil {
					id = d
				}
				return false
			})
			if id != nil && id.Kind() == wrapperchecker.KindIdentifier {
				paramName = id.SourceText()
			}
		}
		body = c
		return false
	})
	if paramName == "" || body == nil {
		return false
	}
	if body.Kind() == wrapperchecker.KindIdentifier {
		return body.SourceText() == paramName
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
