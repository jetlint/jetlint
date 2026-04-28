// Package nomisusedspread implements the no-misused-spread rule:
// flag spreading a string into an array literal — the per-character
// expansion is rarely what the author intended.
package nomisusedspread

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-misused-spread"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSpreadElement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	if parent.Kind() != wrapperchecker.KindArrayLiteralExpression {
		// Object-spread checks (non-objects, function values, etc.) are
		// not yet implemented.
		return
	}
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isStringSpread(t) {
		ctx.Report(n, "spreading a string into an array iterates per-character; use Array.from or .split() if that's intentional")
	}
}

// isStringSpread reports whether t is a string (or primitive
// intersection / union with string) without also being an array. The
// rule only flags when the value is unambiguously a string — a
// `string | number[]` union still has a non-string array branch
// where spread is legitimate, but upstream still flags it.
func isStringSpread(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsStringLike() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsStringLike() {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if m.IsStringLike() {
				return true
			}
		}
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isStringSpread(c)
		}
	}
	return false
}
