// Package noselfassign implements the no-self-assign rule: flag
// assignments whose right-hand side reduces to the same reference
// as the left-hand side (`a = a`, `obj.a = obj.a`, `[a, b] = [a, b]`,
// `({a} = {a})`). The assignment is a no-op and almost always a
// refactoring leftover.
//
// Operators
//   - `=`        — always a no-op when both sides match.
//   - `&&=`/`||=`/`??=` — these short-circuit so `a &&= a`, `a ||= a`,
//     and `a ??= a` are all effectively no-ops. `&=`, `|=` and the
//     other arithmetic compound assignments coerce to a number and
//     so are NOT no-ops; they are left alone.
//
// Options
//   - Props (default true): when true, also flag property/element
//     access self-assignments (`obj.a = obj.a`, `obj[a] = obj[a]`).
//     When false, only flag identifier-to-identifier patterns.
//
// Destructuring is handled element-by-element for arrays and
// property-by-property for objects, including computed keys that
// resolve to the same constant. Spread elements limit how far the
// rule reasons — after a spread on the right of an array, or before
// the last spread on the right of an object, references can no
// longer be statically aligned. This mirrors oxlint and ESLint.
package noselfassign

import (
	"encoding/json"
	"fmt"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-self-assign"

// Options is the configurable surface of no-self-assign.
type Options struct {
	Props bool
}

func DefaultOptions() Options { return Options{Props: true} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-self-assign options must be a JSON object: %w", err)
	}
	if v, ok := fields["props"]; ok {
		if err := json.Unmarshal(v, &out.Props); err != nil {
			return Options{}, err
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
	default:
		return
	}
	r.eachSelfAssignment(ctx, n.BinaryLeft(), n.BinaryRight())
}

// eachSelfAssignment is the main recursive matcher. It walks paired
// LHS/RHS subtrees, reporting whenever a target is being assigned the
// same value it already holds.
func (r *rule) eachSelfAssignment(ctx *engine.Context, left, right *wrapperchecker.Node) {
	left = innerExpr(left)
	right = innerExpr(right)
	if left == nil || right == nil {
		return
	}
	switch left.Kind() {
	case wrapperchecker.KindArrayLiteralExpression:
		if right.Kind() != wrapperchecker.KindArrayLiteralExpression {
			return
		}
		r.matchArray(ctx, left, right)
	case wrapperchecker.KindObjectLiteralExpression:
		if right.Kind() != wrapperchecker.KindObjectLiteralExpression {
			return
		}
		r.matchObject(ctx, left, right)
	default:
		// Simple target: identifier or member expression.
		if right.Kind() == wrapperchecker.KindIdentifier &&
			left.Kind() == wrapperchecker.KindIdentifier &&
			left.LiteralText() == right.LiteralText() {
			ctx.Report(right, "assigning a variable to itself is a no-op")
			return
		}
		if isMemberExpr(left) && isMemberExpr(right) && r.isMemberSameRef(left, right) {
			ctx.Report(right, "assigning to a property the value it already holds is a no-op")
		}
	}
}

// matchArray pairs up elements positionally until a spread element
// appears on the RHS — after that, indices on the LHS no longer line
// up with concrete RHS expressions and we stop.
func (r *rule) matchArray(ctx *engine.Context, leftArr, rightArr *wrapperchecker.Node) {
	leftEls := leftArr.ArrayElements()
	rightEls := rightArr.ArrayElements()
	end := min(len(leftEls), len(rightEls))
	for i := range end {
		l := leftEls[i]
		rt := rightEls[i]
		// Elisions show up as OmittedExpression on the left or just no
		// expression on the right; both prevent matching at this index.
		if l != nil && l.Kind() != wrapperchecker.KindOmittedExpression &&
			rt != nil && rt.Kind() != wrapperchecker.KindOmittedExpression &&
			rt.Kind() != wrapperchecker.KindSpreadElement {
			r.eachSelfAssignment(ctx, assignmentTarget(l), rt)
		}
		if rt != nil && rt.Kind() == wrapperchecker.KindSpreadElement {
			return
		}
	}
}

// matchObject implements the start_j cutoff from oxlint: properties
// before the last spread on the RHS can be overwritten, so the rule
// only pairs LHS properties against RHS properties at or after the
// last spread.
func (r *rule) matchObject(ctx *engine.Context, leftObj, rightObj *wrapperchecker.Node) {
	leftProps := leftObj.ObjectProperties()
	rightProps := rightObj.ObjectProperties()
	if len(rightProps) == 0 {
		return
	}
	startJ := 0
	for k := len(rightProps); k >= 1; k-- {
		if rightProps[k-1].Kind() == wrapperchecker.KindSpreadAssignment {
			startJ = k
			break
		}
	}
	for _, lp := range leftProps {
		for j := startJ; j < len(rightProps); j++ {
			r.eachPropertySelfAssign(ctx, lp, rightProps[j])
		}
	}
}

// eachPropertySelfAssign handles the two property forms that can
// appear as destructuring targets: shorthand (`{a}`) and full
// (`{key: target}`). The right side is always a real object property,
// not a destructuring target.
func (r *rule) eachPropertySelfAssign(ctx *engine.Context, left, right *wrapperchecker.Node) {
	if right.Kind() != wrapperchecker.KindPropertyAssignment &&
		right.Kind() != wrapperchecker.KindShorthandPropertyAssignment {
		return
	}
	rightName, rightHasName := staticPropertyName(right)
	rightValue := propertyValue(right)
	if rightValue == nil {
		return
	}
	switch left.Kind() {
	case wrapperchecker.KindShorthandPropertyAssignment:
		// `{a}` on the LHS — accept only when the LHS has no default.
		if hasInitializer(left) {
			return
		}
		bindingNameNode := left.FirstChild()
		if bindingNameNode == nil || bindingNameNode.Kind() != wrapperchecker.KindIdentifier {
			return
		}
		bindingName := bindingNameNode.LiteralText()
		if !rightHasName || rightName != bindingName {
			return
		}
		rv := innerExpr(rightValue)
		if rv == nil || rv.Kind() != wrapperchecker.KindIdentifier ||
			rv.LiteralText() != bindingName {
			return
		}
		ctx.Report(right, "destructuring property assigns the same identifier to itself")
	case wrapperchecker.KindPropertyAssignment:
		leftName, leftHasName := staticPropertyName(left)
		if !leftHasName || !rightHasName || leftName != rightName {
			return
		}
		r.eachSelfAssignment(ctx, propertyValue(left), rightValue)
	}
}

// isSameRef reports whether two arbitrary expressions reference the
// same runtime value: matching identifiers, this/super tokens, or
// recursively-equal member expressions.
func (r *rule) isSameRef(a, b *wrapperchecker.Node) bool {
	a = innerExpr(a)
	b = innerExpr(b)
	if a == nil || b == nil {
		return false
	}
	if a.Kind() == wrapperchecker.KindThisKeyword && b.Kind() == wrapperchecker.KindThisKeyword {
		return true
	}
	if a.Kind() == wrapperchecker.KindSuperKeyword && b.Kind() == wrapperchecker.KindSuperKeyword {
		return true
	}
	if a.Kind() == wrapperchecker.KindIdentifier && b.Kind() == wrapperchecker.KindIdentifier {
		return a.LiteralText() == b.LiteralText()
	}
	if isMemberExpr(a) && isMemberExpr(b) {
		return r.isMemberSameRef(a, b)
	}
	return false
}

// isMemberSameRef reports whether two member expressions point at the
// same property of the same receiver. With `props: false`, the rule
// stays silent on all member expressions.
func (r *rule) isMemberSameRef(a, b *wrapperchecker.Node) bool {
	if !r.opts.Props {
		return false
	}
	aName, aHasName := staticMemberName(a)
	bName, bHasName := staticMemberName(b)
	if aHasName && bHasName && aName == bName {
		return r.isSameRef(memberReceiver(a), memberReceiver(b))
	}
	aComputed := a.Kind() == wrapperchecker.KindElementAccessExpression
	bComputed := b.Kind() == wrapperchecker.KindElementAccessExpression
	if aComputed != bComputed {
		return false
	}
	if !r.isSameRef(memberReceiver(a), memberReceiver(b)) {
		return false
	}
	if aComputed && bComputed {
		return r.isSameRef(elementIndex(a), elementIndex(b))
	}
	// Both non-computed — covers PropertyAccessExpression with
	// matching private identifier (`#field`). Static-name path above
	// already handled regular property access; this branch fires
	// when both sides have private identifiers.
	aPriv := privateAccessName(a)
	bPriv := privateAccessName(b)
	return aPriv != "" && aPriv == bPriv
}

// innerExpr peels off ParenthesizedExpression wrappers. It does NOT
// descend into comma expressions — destructuring patterns can't
// contain those at the positions we care about.
func innerExpr(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// assignmentTarget unwraps array/object pattern wrappers around a
// nested target. Default-value patterns (`a = 1` in `[a = 1] = ...`)
// show up as BinaryExpression with `=`; the rule treats those as
// having default initializers and skips them, so we return nil.
func assignmentTarget(n *wrapperchecker.Node) *wrapperchecker.Node {
	n = innerExpr(n)
	if n == nil {
		return nil
	}
	if n.Kind() == wrapperchecker.KindBinaryExpression &&
		n.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
		// `[a = 1] = [a]` — LHS has a default, skip.
		return nil
	}
	return n
}

func isMemberExpr(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	return n.Kind() == wrapperchecker.KindPropertyAccessExpression ||
		n.Kind() == wrapperchecker.KindElementAccessExpression
}

func memberReceiver(n *wrapperchecker.Node) *wrapperchecker.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return n.PropertyAccessReceiver()
	case wrapperchecker.KindElementAccessExpression:
		var first *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			first = c
			return true
		})
		return first
	}
	return nil
}

func elementIndex(n *wrapperchecker.Node) *wrapperchecker.Node {
	if n == nil || n.Kind() != wrapperchecker.KindElementAccessExpression {
		return nil
	}
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
			return false
		}
		second = c
		return true
	})
	return second
}

// staticMemberName returns the name being accessed if it resolves to
// a string statically: identifier name for `a.b`, string/number for
// `a['b']` / `a[1]`. Returns ok=false for non-static keys (`a[expr]`)
// and for private identifier access.
func staticMemberName(n *wrapperchecker.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		name := n.PropertyAccessName()
		if strings.HasPrefix(name, "#") {
			return "", false
		}
		return name, true
	case wrapperchecker.KindElementAccessExpression:
		return literalKey(elementIndex(n))
	}
	return "", false
}

func privateAccessName(n *wrapperchecker.Node) string {
	if n == nil || n.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return ""
	}
	name := n.PropertyAccessName()
	if strings.HasPrefix(name, "#") {
		return name
	}
	return ""
}

// staticPropertyName resolves the name of an object literal property,
// covering identifier keys, string/number keys, no-substitution
// templates, BigInts, and constant computed keys.
func staticPropertyName(prop *wrapperchecker.Node) (string, bool) {
	first := prop.FirstChild()
	if first == nil {
		return "", false
	}
	if isStaticNameKind(first.Kind()) {
		return staticTextOf(first), true
	}
	// Computed name: descend.
	return literalKey(first.FirstChild())
}

func literalKey(n *wrapperchecker.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindNumericLiteral:
		return n.LiteralText(), true
	case wrapperchecker.KindBigIntLiteral:
		return strings.TrimSuffix(n.LiteralText(), "n"), true
	case wrapperchecker.KindRegularExpressionLiteral:
		return n.LiteralText(), true
	}
	return "", false
}

func staticTextOf(n *wrapperchecker.Node) string {
	switch n.Kind() {
	case wrapperchecker.KindBigIntLiteral:
		return strings.TrimSuffix(n.LiteralText(), "n")
	}
	return n.LiteralText()
}

func isStaticNameKind(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindPrivateIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return true
	}
	return false
}

// propertyValue returns the value side of a PropertyAssignment or
// the binding identifier of a ShorthandPropertyAssignment (the
// shorthand value is the same name as the key).
func propertyValue(prop *wrapperchecker.Node) *wrapperchecker.Node {
	switch prop.Kind() {
	case wrapperchecker.KindShorthandPropertyAssignment:
		return prop.FirstChild()
	case wrapperchecker.KindPropertyAssignment:
		var name, value *wrapperchecker.Node
		prop.ForEachChild(func(c *wrapperchecker.Node) bool {
			if name == nil {
				name = c
				return false
			}
			value = c
			return true
		})
		return value
	}
	return nil
}

// hasInitializer detects the `{a = 1}` default-value form on a
// ShorthandPropertyAssignment. The rule skips destructuring shapes
// with defaults — the assignment changes behavior when the property
// is missing.
func hasInitializer(prop *wrapperchecker.Node) bool {
	count := 0
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		count++
		return false
	})
	return count > 1
}
