// Package nounsafecall implements the no-unsafe-call rule: flag `x()`
// where x has type `any`.
package nounsafecall

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-call"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:           visit,
		wrapperchecker.KindNewExpression:            visit,
		wrapperchecker.KindTaggedTemplateExpression: visitTagged,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	// `import('./mod')` — callee is the import keyword, a synthetic
	// global. Not a runtime any value.
	if callee.Kind() == wrapperchecker.KindImportKeyword {
		return
	}
	t := ctx.TypeOf(callee)
	if t == nil {
		return
	}
	isNew := n.Kind() == wrapperchecker.KindNewExpression
	if isUnsafeCallee(t, isNew) {
		ctx.Report(callee, "calling a value of type `any` (or bare Function) defeats the type checker")
	}
}

func visitTagged(ctx *engine.Context, n *wrapperchecker.Node) {
	tag := n.FirstChild()
	if tag == nil {
		return
	}
	t := ctx.TypeOf(tag)
	if t == nil {
		return
	}
	if isUnsafeCallee(t, false) {
		ctx.Report(tag, "calling a value of type `any` (or bare Function) defeats the type checker")
	}
}

// isUnsafeCallee reports whether the callee's type is `any` or the
// bare `Function` interface (a value that can be called but without a
// specific signature). Interfaces extending Function with their own
// call signatures are safe to call; with their own construct
// signatures, safe to `new`.
func isUnsafeCallee(t *wrapperchecker.Type, isNew bool) bool {
	if t.IsAny() {
		return true
	}
	if t.SymbolName() == "Function" {
		return true
	}
	hasFunctionBase := false
	for _, base := range t.BaseTypeNames() {
		if base == "Function" {
			hasFunctionBase = true
			break
		}
	}
	if !hasFunctionBase {
		return false
	}
	// Subtype of Function — safe only if it has the relevant signature
	// kind for the call shape we're checking.
	if isNew {
		return len(t.ConstructSignatures()) == 0
	}
	return len(t.CallSignatures()) == 0
}
