// Package novoidtypereturn implements no-void-type-return: when a
// function is annotated `: void`, it should not return a value.
// A bare `return;` is fine, and `return void X;` is fine (the
// expression yields void). Anything else (`return undefined`, a
// real value) is flagged — the `: void` annotation promises no
// useful return value, so an actual one is almost certainly a
// type-vs-implementation mismatch.
package novoidtypereturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-void-type-return"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	ret := n.FunctionReturnTypeAnnotation()
	if ret == nil || ret.Kind() != wrapperchecker.KindVoidKeyword {
		return
	}
	walkReturns(n, n, func(r *wrapperchecker.Node) {
		expr := returnExpression(r)
		if expr == nil {
			return
		}
		if expr.Kind() == wrapperchecker.KindVoidExpression {
			return
		}
		ctx.Report(r, "function is declared `: void` — drop the returned value")
	})
}

// walkReturns invokes visit for each ReturnStatement lexically
// enclosed by fn, without descending into nested functions.
// Returns inside an inner FunctionExpression / ArrowFunction belong
// to that inner function's return type, not fn's.
func walkReturns(fn, n *wrapperchecker.Node, visitRet func(*wrapperchecker.Node)) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c == fn {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return false
		case wrapperchecker.KindReturnStatement:
			visitRet(c)
			return false
		}
		walkReturns(fn, c, visitRet)
		return false
	})
}

// returnExpression returns the optional argument of a ReturnStatement,
// or nil for a bare `return;`. ReturnStatement has at most one child
// in the wrapper AST.
func returnExpression(ret *wrapperchecker.Node) *wrapperchecker.Node {
	var got *wrapperchecker.Node
	ret.ForEachChild(func(c *wrapperchecker.Node) bool {
		got = c
		return true
	})
	return got
}
