// Package strictbooleanexpressions implements the strict-boolean-expressions
// rule: flag any boolean-context expression whose type is not strictly
// boolean. Common offenders are nullable strings used in `if (x)` style
// truthiness checks, where the developer may have meant to compare to
// something more specific.
package strictbooleanexpressions

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "strict-boolean-expressions"

// New constructs a fresh rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           visitTestPosition,
		wrapperchecker.KindConditionalExpression: visitTestPosition,
		wrapperchecker.KindWhileStatement:        visitTestPosition,
	}
}

// visitTestPosition checks the first child of n, which by tsgo's AST
// generation is always the test/condition expression for these node
// kinds.
func visitTestPosition(ctx *engine.Context, n *wrapperchecker.Node) {
	test := n.FirstChild()
	if test == nil {
		return
	}
	t := ctx.TypeOf(test)
	if t == nil {
		return
	}
	if isStrictlyBoolean(t) {
		return
	}
	if t.IsAny() || t.IsUnknown() {
		// `any` and `unknown` are deliberately excluded; the
		// no-unsafe-* family handles `any` flow, and `unknown` requires
		// an explicit narrowing before it can be tested.
		ctx.Report(test,
			"boolean test on a value of type any or unknown; narrow the value first")
		return
	}
	ctx.Report(test,
		"boolean test on a value whose type is not strictly boolean; coerce explicitly or compare against the intended sentinel")
}

// isStrictlyBoolean reports whether t is acceptable as a boolean test
// per the rule's defaults: a strictly boolean type, or a type whose
// every member is a "safe" coercion target (boolean, string, or
// number). Nullables and other oddities are not safe — the whole point
// of the rule is to surface implicit nullability checks. This matches
// typescript-eslint's defaults of allowString=true, allowNumber=true,
// allowNullableString=false, allowNullableNumber=false.
func isStrictlyBoolean(t *wrapperchecker.Type) bool {
	if !t.IsUnion() {
		return memberIsAcceptable(t)
	}
	for _, m := range t.UnionMembers() {
		if !memberIsAcceptable(m) {
			return false
		}
	}
	return true
}

func memberIsAcceptable(m *wrapperchecker.Type) bool {
	return m.IsBooleanLike() || m.IsStringLike() || isNumberLike(m)
}

// isNumberLike reports whether the type is number-shaped. The wrapper
// does not yet expose an IsNumberLike helper; we approximate with the
// String() rendering until a real predicate is needed elsewhere.
func isNumberLike(t *wrapperchecker.Type) bool {
	s := t.String()
	return s == "number" || s == "bigint"
}
