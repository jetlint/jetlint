// Package nounsafetypeassertion implements the no-unsafe-type-assertion
// rule: flag `x as T` where T is neither a supertype nor a subtype of
// x's type — the cast bypasses type-checking entirely.
package nounsafetypeassertion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-type-assertion"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAsExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.AsExpressionSource()
	annot := n.AsExpressionTarget()
	if src == nil || annot == nil {
		return
	}
	srcT := ctx.TypeOf(src)
	target := ctx.Checker().TypeFromTypeNode(annot)
	if srcT == nil || target == nil {
		return
	}
	if srcT.IsAny() || srcT.IsUnknown() || target.IsAny() || target.IsUnknown() {
		return
	}
	// Safe direction: widening (`narrower as wider`). The source's
	// type is already assignable to the target.
	if srcT.IsAssignableTo(target) {
		return
	}
	ctx.Report(n, "type assertion narrows or sidesteps — the source's type isn't assignable to the asserted type")
}
