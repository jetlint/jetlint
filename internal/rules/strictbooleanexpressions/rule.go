// Package strictbooleanexpressions implements the strict-boolean-expressions
// rule: flag any boolean-context expression whose type is not strictly
// boolean.
package strictbooleanexpressions

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "strict-boolean-expressions"

// Options is the configurable surface of the rule.
type Options struct {
	AllowString          bool
	AllowNumber          bool
	AllowNullableObject  bool
	AllowNullableBoolean bool
	AllowNullableString  bool
	AllowNullableNumber  bool
	AllowNullableEnum    bool
	AllowAny             bool
	AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing bool
}

func DefaultOptions() Options {
	return Options{
		AllowString:         true,
		AllowNumber:         true,
		AllowNullableObject: true,
	}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("strict-boolean-expressions options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowString":
			if err := json.Unmarshal(val, &out.AllowString); err != nil {
				return Options{}, err
			}
		case "allowNumber":
			if err := json.Unmarshal(val, &out.AllowNumber); err != nil {
				return Options{}, err
			}
		case "allowNullableObject":
			if err := json.Unmarshal(val, &out.AllowNullableObject); err != nil {
				return Options{}, err
			}
		case "allowNullableBoolean":
			if err := json.Unmarshal(val, &out.AllowNullableBoolean); err != nil {
				return Options{}, err
			}
		case "allowNullableString":
			if err := json.Unmarshal(val, &out.AllowNullableString); err != nil {
				return Options{}, err
			}
		case "allowNullableNumber":
			if err := json.Unmarshal(val, &out.AllowNullableNumber); err != nil {
				return Options{}, err
			}
		case "allowNullableEnum":
			if err := json.Unmarshal(val, &out.AllowNullableEnum); err != nil {
				return Options{}, err
			}
		case "allowAny":
			if err := json.Unmarshal(val, &out.AllowAny); err != nil {
				return Options{}, err
			}
		}
	}
	return out, nil
}

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           r.visitTestPosition,
		wrapperchecker.KindConditionalExpression: r.visitTestPosition,
		wrapperchecker.KindWhileStatement:        r.visitTestPosition,
		wrapperchecker.KindDoStatement:           r.visitTestPosition,
		wrapperchecker.KindForStatement:          r.visitForStatement,
		wrapperchecker.KindPrefixUnaryExpression: r.visitPrefixUnary,
		wrapperchecker.KindBinaryExpression:      r.visitBinary,
	}
}

// visitBinary checks the left operand of `&&` and `||` — the
// short-circuit operators evaluate their LHS as a boolean test, so
// the same constraints apply. The right operand is not in a boolean
// position (its value flows out as the expression result), so it
// isn't checked here.
func (r *rule) visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindAmpersandAmpersandToken, wrapperchecker.KindBarBarToken:
	default:
		return
	}
	// Skip nested `a && b && c` — the outermost binary's LHS chain
	// will recursively visit each component when the engine fires on
	// the inner binaries.
	if parent := n.Parent(); parent != nil && parent.Kind() == wrapperchecker.KindBinaryExpression {
		switch parent.BinaryOperatorKind() {
		case wrapperchecker.KindAmpersandAmpersandToken, wrapperchecker.KindBarBarToken:
			return
		}
	}
	left := n.BinaryLeft()
	if left == nil {
		return
	}
	r.checkBoolean(ctx, left)
}

func (r *rule) visitForStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	cond := n.ForStatementCondition()
	if cond == nil {
		return
	}
	r.checkBoolean(ctx, cond)
}

func (r *rule) visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	r.checkBoolean(ctx, operand)
}

func (r *rule) checkBoolean(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindBarBarToken:
			if l := expr.BinaryLeft(); l != nil {
				r.checkBoolean(ctx, l)
			}
			if rr := expr.BinaryRight(); rr != nil {
				r.checkBoolean(ctx, rr)
			}
			return
		}
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if r.isAcceptable(t) {
		return
	}
	if t.IsAny() || t.IsUnknown() {
		ctx.Report(expr, "boolean test on a value of type any or unknown; narrow the value first")
		return
	}
	ctx.Report(expr, "boolean test on a value whose type is not strictly boolean; coerce explicitly or compare against the intended sentinel")
}

func (r *rule) visitTestPosition(ctx *engine.Context, n *wrapperchecker.Node) {
	test := testExpressionOf(n)
	if test == nil {
		return
	}
	r.checkBoolean(ctx, test)
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

// isAlwaysTruthyLiteral reports whether t is a literal type that can
// never be falsy at runtime — non-empty strings, non-zero numbers,
// the `true` literal, etc.
func isAlwaysTruthyLiteral(t *wrapperchecker.Type) bool {
	s := t.String()
	switch {
	case t.IsStringLike() && s != "string" && s != "\"\"" && s != "''":
		return true
	case t.IsNumberLike() && s != "number" && s != "0":
		return true
	case t.IsBigIntLike() && s != "bigint" && s != "0n":
		return true
	case t.IsBooleanLike() && s == "true":
		return true
	}
	return false
}

// isAcceptable reports whether t is OK in a boolean test under the
// configured options.
func (r *rule) isAcceptable(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return r.opts.AllowAny
	}
	if !t.IsUnion() {
		return r.scalarAcceptable(t)
	}
	hasNullable := false
	hasBool := false
	hasString := false
	hasNumber := false
	hasEnum := false
	hasOther := false
	hasAlwaysTruthy := false
	for _, m := range t.UnionMembers() {
		switch {
		case m.IsNullOrUndefined():
			hasNullable = true
		case m.IsEnumLike():
			// Check enum membership before the more general string/
			// number primitive checks — an enum literal is also
			// IsNumberLike/IsStringLike but the enum identity is
			// the load-bearing classification for nullable-enum
			// option handling.
			hasEnum = true
		case isAlwaysTruthyLiteral(m):
			// Non-empty string, non-zero numeric, `true`, etc. — never
			// confused with the null/undefined branch in a boolean test.
			hasAlwaysTruthy = true
		case m.IsBooleanLike() || intersectionContainsBoolean(m):
			hasBool = true
		case m.IsStringLike() || intersectionContainsStringLike(m):
			hasString = true
		case m.IsNumberLike() || m.IsBigIntLike() || intersectionContainsNumberLike(m):
			hasNumber = true
		case m.IsNever():
			// Unreachable.
		case m.IsTypeParameter():
			if c := m.BaseConstraint(); c != nil && c != m && r.isAcceptable(c) {
				continue
			}
			hasOther = true
		default:
			hasOther = true
		}
	}
	// If the only non-nullable members are always-truthy literals (or
	// always-truthy literals + an object), treat the whole union the
	// same way as `T | null` for an object T.
	if hasAlwaysTruthy && !hasBool && !hasString && !hasNumber && !hasEnum && !hasOther {
		if hasNullable {
			return r.opts.AllowNullableObject
		}
		return true
	}
	if hasAlwaysTruthy {
		// Mixed always-truthy with regular primitives — keep checking
		// the regular primitives below; the literal doesn't add risk.
	}
	if hasOther {
		// Object/function-typed members count as "nullable-object" only
		// when paired with a nullable.
		return hasNullable && r.opts.AllowNullableObject &&
			!hasBool && !hasString && !hasNumber && !hasEnum
	}
	if hasNullable {
		switch {
		case hasBool && !hasString && !hasNumber && !hasEnum:
			return r.opts.AllowNullableBoolean
		case hasString && !hasBool && !hasNumber && !hasEnum:
			return r.opts.AllowNullableString && r.opts.AllowString
		case hasNumber && !hasBool && !hasString && !hasEnum:
			return r.opts.AllowNullableNumber && r.opts.AllowNumber
		case hasEnum && !hasBool && !hasString && !hasNumber:
			return r.opts.AllowNullableEnum
		case !hasBool && !hasString && !hasNumber && !hasEnum:
			// Pure null/undefined union — the condition is always
			// false; flag regardless of AllowNullableObject (which is
			// about nullable object refs, not the no-object case).
			return false
		}
		return false
	}
	// No nullable.
	if hasBool && !hasString && !hasNumber && !hasEnum {
		return true
	}
	if hasString && !hasBool && !hasNumber && !hasEnum {
		return r.opts.AllowString
	}
	if hasNumber && !hasBool && !hasString && !hasEnum {
		return r.opts.AllowNumber
	}
	if hasBool || hasString || hasNumber || hasEnum {
		// Mixed primitive union — only OK if all components are.
		if hasBool {
			if hasString && !r.opts.AllowString {
				return false
			}
			if hasNumber && !r.opts.AllowNumber {
				return false
			}
			return true
		}
	}
	return false
}

func intersectionContainsBoolean(t *wrapperchecker.Type) bool {
	if !t.IsIntersection() {
		return false
	}
	for _, m := range t.IntersectionMembers() {
		if m.IsBooleanLike() {
			return true
		}
	}
	return false
}

func intersectionContainsStringLike(t *wrapperchecker.Type) bool {
	if !t.IsIntersection() {
		return false
	}
	for _, m := range t.IntersectionMembers() {
		if m.IsStringLike() {
			return true
		}
	}
	return false
}

func intersectionContainsNumberLike(t *wrapperchecker.Type) bool {
	if !t.IsIntersection() {
		return false
	}
	for _, m := range t.IntersectionMembers() {
		if m.IsNumberLike() || m.IsBigIntLike() {
			return true
		}
	}
	return false
}

func (r *rule) scalarAcceptable(t *wrapperchecker.Type) bool {
	if t.IsBooleanLike() {
		return true
	}
	if t.IsStringLike() {
		return r.opts.AllowString
	}
	if t.IsNumberLike() || t.IsBigIntLike() {
		return r.opts.AllowNumber
	}
	if t.IsNever() {
		return true
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return r.isAcceptable(c)
		}
	}
	// Branded intersections (`boolean & { __BRAND }`) keep the
	// flavor of their primitive member — accept the intersection if
	// any member is itself acceptable in a boolean position.
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if r.scalarAcceptable(m) {
				return true
			}
		}
	}
	return false
}
