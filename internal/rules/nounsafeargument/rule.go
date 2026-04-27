// Package nounsafeargument implements the no-unsafe-argument rule:
// flag `f(x)` where x has type any but the parameter expects something
// more specific.
package nounsafeargument

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-argument"

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
	args := n.CallArguments()
	for i, arg := range args {
		argT := ctx.TypeOf(arg)
		if argT == nil || !argT.IsAny() {
			continue
		}
		paramT := ctx.Checker().ContextualTypeForArgument(n, i)
		if paramT == nil {
			continue
		}
		if paramT.IsAny() || paramT.IsUnknown() {
			continue
		}
		ctx.Report(arg, "passing an `any` value to a parameter with a more specific type")
	}
}

// visitTagged handles `tag` template `${a}${b}` — each interpolation is
// an argument to the tag function, with parameter slot offset by one
// (the first parameter is the TemplateStringsArray).
func visitTagged(ctx *engine.Context, n *wrapperchecker.Node) {
	args := n.TaggedTemplateInterpolations()
	for i, arg := range args {
		argT := ctx.TypeOf(arg)
		if argT == nil || !argT.IsAny() {
			continue
		}
		paramT := ctx.Checker().ContextualTypeForArgument(n, i+1)
		if paramT == nil {
			continue
		}
		if paramT.IsAny() || paramT.IsUnknown() {
			continue
		}
		ctx.Report(arg, "passing an `any` value to a parameter with a more specific type")
	}
}
