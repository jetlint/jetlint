// Package nounsafeunaryminus implements the no-unsafe-unary-minus rule:
// flag `-x` where x's type is not a number, bigint, any, or never.
package nounsafeunaryminus

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-unary-minus"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPrefixUnaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "-" {
		return
	}
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	t := ctx.TypeOf(operand)
	if t == nil {
		return
	}
	if isNumericLike(t, 0) {
		return
	}
	ctx.Report(n, "unary minus on a value whose type is not numeric coerces to NaN; the result is almost certainly unintended")
}

const recursionLimit = 16

func isNumericLike(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return true
	}
	if t.IsAny() || t.IsNever() {
		return true
	}
	// number | bigint
	s := t.String()
	if s == "number" || s == "bigint" {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isNumericLike(m, depth+1) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		// Any numeric member qualifies the intersection.
		for _, m := range t.IntersectionMembers() {
			if isNumericLike(m, depth+1) {
				return true
			}
		}
		return false
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return isNumericLike(c, depth+1)
	}
	return false
}
