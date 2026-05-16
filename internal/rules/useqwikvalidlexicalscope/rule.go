// Package useqwikvalidlexicalscope implements
// use-qwik-valid-lexical-scope: Qwik's `$(...)` marker turns the
// wrapped function into a serializable QRL that the runtime can
// resume on a different worker. Calling it outside a `component$`
// factory means the QRL captures lexical state Qwik can't serialize
// at the right boundary — the closure may run, but Qwik can't
// guarantee its captures are reachable. The rule fires on bare
// `$(...)` calls that aren't inside a `component$(...)` factory.
package useqwikvalidlexicalscope

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-qwik-valid-lexical-scope"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, call *wrapperchecker.Node) {
	if !isDollarCall(call) {
		return
	}
	if insideComponentFactory(call) {
		return
	}
	ctx.Report(call, "`$(...)` must be called inside a `component$(...)` factory so Qwik can serialize the captured scope")
}

func isDollarCall(call *wrapperchecker.Node) bool {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	return callee.LiteralText() == "$"
}

func insideComponentFactory(call *wrapperchecker.Node) bool {
	for p := call.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionExpression:
			if functionIsComponentFactoryArg(p) {
				return true
			}
		}
	}
	return false
}

func functionIsComponentFactoryArg(fn *wrapperchecker.Node) bool {
	p := fn.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil || p.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := p.CalleeExpression()
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		name := callee.LiteralText()
		return name == "component$"
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName() == "component$"
	}
	return false
}
