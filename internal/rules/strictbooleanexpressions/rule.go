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
		wrapperchecker.KindDoStatement:           visitTestPosition,
		wrapperchecker.KindForStatement:          visitForStatement,
		wrapperchecker.KindPrefixUnaryExpression: visitPrefixUnary,
	}
}

func visitForStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	cond := n.ForStatementCondition()
	if cond == nil {
		return
	}
	checkBoolean(ctx, cond)
}

func visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	checkBoolean(ctx, operand)
}

// checkBoolean reports the expression if its type isn't strictly
// boolean. Descends into `&&`/`||` operands so each branch of a
// short-circuit chain is checked at its truthiness position.
func checkBoolean(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindBarBarToken:
			if l := expr.BinaryLeft(); l != nil {
				checkBoolean(ctx, l)
			}
			if r := expr.BinaryRight(); r != nil {
				checkBoolean(ctx, r)
			}
			return
		}
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isStrictlyBoolean(t) {
		return
	}
	if t.IsAny() || t.IsUnknown() {
		ctx.Report(expr, "boolean test on a value of type any or unknown; narrow the value first")
		return
	}
	ctx.Report(expr, "boolean test on a value whose type is not strictly boolean; coerce explicitly or compare against the intended sentinel")
}

// visitTestPosition checks the test/condition expression of the
// given node, descending into short-circuit operators so each branch
// is verified at its truthiness position.
func visitTestPosition(ctx *engine.Context, n *wrapperchecker.Node) {
	test := testExpressionOf(n)
	if test == nil {
		return
	}
	checkBoolean(ctx, test)
}

func testExpressionOf(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindIfStatement:
		return n.IfCondition()
	case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement:
		return n.WhileCondition()
	case wrapperchecker.KindConditionalExpression:
		return n.ConditionalCondition()
	}
	return nil
}

// isStrictlyBoolean reports whether t is acceptable as a boolean test
// per the rule's defaults: a strictly boolean type, or a type whose
// every member is a "safe" coercion target (boolean, string, or
// number). Nullables and other oddities are not safe — the whole point
// of the rule is to surface implicit nullability checks. This matches
// typescript-eslint's defaults of allowString=true, allowNumber=true,
// allowNullableString=false, allowNullableNumber=false.
func isStrictlyBoolean(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !memberIsAcceptable(m) {
				return false
			}
		}
		return true
	}
	return memberIsAcceptable(t)
}

func memberIsAcceptable(m *wrapperchecker.Type) bool {
	if m.IsBooleanLike() || m.IsStringLike() || m.IsNumberLike() || m.IsBigIntLike() {
		return true
	}
	// `never` is unreachable — testing it can't actually trigger the
	// rule's concerns about implicit coercion.
	if m.IsNever() {
		return true
	}
	// Generic type parameter: defer to its base constraint.
	if m.IsTypeParameter() {
		if c := m.BaseConstraint(); c != nil && c != m {
			return isStrictlyBoolean(c)
		}
	}
	return false
}
