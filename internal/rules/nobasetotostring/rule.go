// Package nobasetotostring implements the no-base-to-string rule:
// flag template-literal interpolations of values whose type does not
// supply its own string conversion (i.e., values that would render as
// "[object Object]"). This is a universally relatable bug class: every
// engineer has shipped "[object Object]" to a log, a UI, or a database
// at some point.
package nobasetotostring

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-base-to-string"

// New constructs a fresh rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTemplateSpan: visitTemplateSpan,
	}
}

func visitTemplateSpan(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if hasMeaningfulStringConversion(t) {
		return
	}
	ctx.Report(expr,
		"interpolating a value whose type does not declare a custom string conversion will produce \"[object Object]\" or a similar default rendering")
}

// hasMeaningfulStringConversion returns true if the type, or every
// member of a union type, declares its own toString or is a primitive.
// Returns false only when ANY constituent will render as the default
// object representation, since one bad branch is sufficient to ship
// garbage at runtime.
func hasMeaningfulStringConversion(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		// `any` and `unknown` are deliberately excluded: the
		// no-unsafe-* family handles `any` flow.
		return true
	}
	if !t.IsUnion() {
		return t.HasOwnToString()
	}
	for _, m := range t.UnionMembers() {
		if !m.HasOwnToString() {
			return false
		}
	}
	return true
}
