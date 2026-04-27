// Package onlythrowerror implements the only-throw-error rule: flag
// `throw X` where X is not an Error (or any/unknown).
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint.
package onlythrowerror

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "only-throw-error"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindThrowStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isAcceptableThrow(t, 0) {
		return
	}
	ctx.Report(expr, "throw of a value whose type is not Error or a subclass")
}

const recursionLimit = 16

func isAcceptableThrow(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return true
	}
	if t.IsAny() || t.IsUnknown() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAcceptableThrow(m, depth+1) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isAcceptableThrow(m, depth+1) {
				return true
			}
		}
		return false
	}
	if name := t.SymbolName(); isErrorName(name) {
		return true
	}
	for _, base := range t.BaseTypeNames() {
		if isErrorName(base) {
			return true
		}
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return isAcceptableThrow(c, depth+1)
	}
	return false
}

// isErrorName reports whether the type name is a built-in Error class
// (or one of the standard subclasses).
func isErrorName(name string) bool {
	switch name {
	case "Error", "TypeError", "RangeError", "SyntaxError",
		"ReferenceError", "URIError", "EvalError",
		"AggregateError":
		return true
	}
	return false
}
