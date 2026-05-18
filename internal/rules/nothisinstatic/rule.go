// Package nothisinstatic implements no-this-in-static: inside a
// static method or static initializer block, `this` refers to the
// class itself, not an instance. Use the class name explicitly so
// the meaning is obvious at the use site.
package nothisinstatic

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-this-in-static"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindThisKeyword: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// `new this(...)` references the class as a constructor — idiomatic.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindNewExpression {
		return
	}
	// Walk ancestors; stop at first function-like boundary that owns `this`.
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression:
			// Regular function — `this` is the function's own binding, not the class's.
			return
		case wrapperchecker.KindMethodDeclaration, wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor, wrapperchecker.KindPropertyDeclaration:
			if hasStaticModifier(p) {
				ctx.Report(n, "`this` in a static context refers to the class — use the class name")
			}
			return
		case wrapperchecker.KindBlock:
			// A "static {}" block inside a class — detected by parent.
			if pp := p.Parent(); pp != nil {
				if pp.Kind() == wrapperchecker.KindClassDeclaration || pp.Kind() == wrapperchecker.KindClassExpression {
					ctx.Report(n, "`this` in a static initializer refers to the class — use the class name")
					return
				}
			}
		case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
			return
		}
	}
}

func hasStaticModifier(n *wrapperchecker.Node) bool {
	out := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStaticKeyword {
			out = true
		}
		return false
	})
	return out
}
