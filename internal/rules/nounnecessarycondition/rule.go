// Package nounnecessarycondition implements the no-unnecessary-condition
// rule: flag conditional positions whose test type is provably
// constant (always-true or always-false).
package nounnecessarycondition

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-condition"

// Options is the configurable surface of the rule.
type Options struct {
	// AllowConstantLoopConditions: when set, `while/for/do` loops with
	// statically-constant test expressions are not flagged. The
	// upstream rule accepts `true | false | "always" | "never" | "only-allowed-literals"`;
	// this implementation treats anything truthy as "always allow".
	AllowConstantLoopConditions bool
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           visitIf,
		wrapperchecker.KindWhileStatement:        r.visitWhile,
		wrapperchecker.KindDoStatement:           r.visitWhile,
		wrapperchecker.KindForStatement:          r.visitFor,
		wrapperchecker.KindConditionalExpression: visitConditional,
		wrapperchecker.KindBinaryExpression:      visitBinary,
		wrapperchecker.KindPrefixUnaryExpression: visitPrefixUnary,
	}
}

func visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	check(ctx, n.FirstChild())
}

// visitBinary covers `a && b` and `a || b` outside an explicit test
// position — the operator's branching depends on `a`'s truthiness,
// so a constant `a` makes the whole expression redundant. `??` is
// excluded because TS doesn't always model index access as nullable
// (the `noUncheckedIndexedAccess` flag changes the type), so a
// trailing `?? default` is often a deliberate runtime guard.
func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken:
	default:
		return
	}
	if l := n.BinaryLeft(); l != nil {
		check(ctx, l)
	}
}

func visitIf(ctx *engine.Context, n *wrapperchecker.Node) {
	checkRecursive(ctx, n.IfCondition())
}

func (r *rule) visitWhile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.AllowConstantLoopConditions {
		return
	}
	checkRecursive(ctx, n.WhileCondition())
}

func (r *rule) visitFor(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.AllowConstantLoopConditions {
		return
	}
	checkRecursive(ctx, n.ForStatementCondition())
}

func visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	checkRecursive(ctx, n.ConditionalCondition())
}

// checkRecursive walks &&/||/?? chains at the test position so each
// operand is checked individually. `b1 && b2` where b1 is always
// truthy reports on b1, since the conjunction collapses to just b2.
func checkRecursive(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindBarBarToken:
			checkRecursive(ctx, expr.BinaryLeft())
			checkRecursive(ctx, expr.BinaryRight())
			return
		}
	}
	if expr.Kind() == wrapperchecker.KindParenthesizedExpression {
		checkRecursive(ctx, expr.FirstChild())
		return
	}
	check(ctx, expr)
}

func check(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isAlwaysTruthy(t) {
		ctx.Report(expr, "condition is always truthy")
		return
	}
	if isAlwaysFalsy(t) {
		ctx.Report(expr, "condition is always falsy")
	}
}

// isAlwaysTruthy reports whether t is a type whose every inhabitant
// is truthy at runtime. Covers `true`, non-empty string literals,
// non-zero number literals, and unions of such.
func isAlwaysTruthy(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAlwaysTruthy(m) {
				return false
			}
		}
		return true
	}
	s := t.String()
	switch {
	case t.IsBooleanLike() && s == "true":
		return true
	case t.IsStringLike() && s != "string" && s != "\"\"" && s != "''":
		return true
	case t.IsNumberLike() && s != "number" && s != "0":
		return true
	case t.IsBigIntLike() && s != "bigint" && s != "0n":
		return true
	}
	// Non-primitive, non-nullable types (objects, arrays, functions,
	// classes) are always truthy in JS — only `null`/`undefined`/empty
	// strings/zero numbers are falsy. We've already excluded those.
	if isNonNullableNonPrimitive(t) {
		return true
	}
	return false
}

func isNonNullableNonPrimitive(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() || t.IsNullOrUndefined() || t.IsNever() || t.IsVoid() {
		return false
	}
	if t.IsBooleanLike() || t.IsStringLike() || t.IsNumberLike() || t.IsBigIntLike() || t.IsEnumLike() {
		return false
	}
	if t.IsTypeParameter() {
		return false
	}
	if t.IsIntersection() {
		// Branded primitives like `boolean & { __brand: string }` look
		// like an intersection but their truth value is governed by
		// the underlying primitive, not the brand.
		for _, m := range t.IntersectionMembers() {
			if m.IsBooleanLike() || m.IsStringLike() || m.IsNumberLike() || m.IsBigIntLike() {
				return false
			}
		}
	}
	return true
}

// isAlwaysFalsy reports whether t can never be truthy at runtime.
func isAlwaysFalsy(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAlwaysFalsy(m) {
				return false
			}
		}
		return true
	}
	if t.IsNullOrUndefined() {
		return true
	}
	s := t.String()
	switch s {
	case "false", "\"\"", "''", "0", "0n":
		return true
	}
	return false
}
