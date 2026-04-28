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
		wrapperchecker.KindSpreadElement:    visit,
		wrapperchecker.KindSpreadAssignment: visitSpreadAssignment,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil {
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
	switch parent.Kind() {
	case wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindCallExpression,
		wrapperchecker.KindNewExpression:
		if isStringSpread(t) {
			ctx.Report(n, "spreading a string iterates per-character; use Array.from or .split() if that's intentional")
		}
	}
}

func visitSpreadAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isArraySpread(t) {
		ctx.Report(n, "spreading an array into an object — array indices become string keys, which is rarely intentional")
		return
	}
	// `{ ...someSet }` and `{ ...someMap }` — the iteration protocol
	// produces tuples, not key/value pairs the object literal can use.
	if name := setOrMapSymbol(t); name != "" {
		ctx.Report(n, "spreading a "+name+" into an object — Set/Map iteration doesn't produce string-keyed properties")
	}
}

// setOrMapSymbol returns the named flavor of Set/Map that t (or any
// union member) carries, or "" when t doesn't reference one. Walks
// unions so `Set<number> | { a: number }` is still flagged.
func setOrMapSymbol(t *wrapperchecker.Type) string {
	if t == nil {
		return ""
	}
	switch t.SymbolName() {
	case "Set", "ReadonlySet", "WeakSet", "Map", "ReadonlyMap", "WeakMap":
		return t.SymbolName()
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if name := setOrMapSymbol(m); name != "" {
				return name
			}
		}
	}
	return ""
}

func isArraySpread(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsTupleType() || t.IsArrayLikeType() || t.ArrayElementType() != nil {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isArraySpread(m) {
				return true
			}
		}
	}
	return false
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
			if isStringSpread(m) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isStringSpread(m) {
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
