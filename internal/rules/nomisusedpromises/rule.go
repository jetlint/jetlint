// Package nomisusedpromises implements the no-misused-promises rule:
// flag any expression where a Promise is used in a position the
// language does not handle (conditionals, void-returning callback
// slots, spreads).
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint. The check shape, option set, and contextual
// type semantics derive from upstream's published test fixtures.
package nomisusedpromises

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-misused-promises"

// Options is the configurable surface of the rule.
type Options struct {
	ChecksConditionals bool
	ChecksVoidReturn   bool
	ChecksSpreads      bool
}

func DefaultOptions() Options {
	return Options{ChecksConditionals: true, ChecksVoidReturn: true, ChecksSpreads: true}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-misused-promises options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "checksConditionals":
			if err := json.Unmarshal(val, &out.ChecksConditionals); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q: %w", key, err)
			}
		case "checksSpreads":
			if err := json.Unmarshal(val, &out.ChecksSpreads); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q: %w", key, err)
			}
		case "checksVoidReturn":
			var b bool
			if err := json.Unmarshal(val, &b); err == nil {
				out.ChecksVoidReturn = b
				continue
			}
			var sub map[string]any
			if err := json.Unmarshal(val, &sub); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q must be boolean or sub-options object: %w", key, err)
			}
			out.ChecksVoidReturn = true
		default:
			return Options{}, fmt.Errorf("no-misused-promises has no option %q (expected checksConditionals, checksVoidReturn, or checksSpreads)", key)
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:        r.visitCallExpression,
		wrapperchecker.KindNewExpression:         r.visitCallExpression,
		wrapperchecker.KindIfStatement:           r.visitConditional,
		wrapperchecker.KindWhileStatement:        r.visitConditional,
		wrapperchecker.KindDoStatement:           r.visitConditional,
		wrapperchecker.KindForStatement:          r.visitForStatement,
		wrapperchecker.KindConditionalExpression: r.visitConditional,
		wrapperchecker.KindBinaryExpression:      r.visitBinaryExpression,
		wrapperchecker.KindPrefixUnaryExpression: r.visitPrefixUnary,
		wrapperchecker.KindVariableDeclaration:        r.visitVariableDeclaration,
		wrapperchecker.KindPropertyAssignment:         r.visitPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment: r.visitShorthandProperty,
		wrapperchecker.KindMethodDeclaration:          r.visitMethodDeclaration,
		wrapperchecker.KindMethodSignature:            r.visitMethodSignature,
		wrapperchecker.KindSpreadElement:         r.visitSpreadElement,
		wrapperchecker.KindSpreadAssignment:      r.visitSpreadElement,
	}
}

const (
	msgVoidCallback = "async callback returns a Promise that the parameter's void return type silently drops"
	msgConditional  = "promise used in a conditional position; the language tests truthiness, not promise resolution"
	msgSpread       = "promise spread does not unwrap into iterable elements"
)

func (r *rule) visitCallExpression(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	args := n.CallArguments()
	for i, arg := range args {
		if !returnsPromise(ctx, arg) {
			continue
		}
		expected := ctx.Checker().ContextualTypeForArgument(n, i)
		if expected == nil {
			continue
		}
		if !allCallSignaturesExpectVoid(expected) {
			continue
		}
		ctx.Report(arg, msgVoidCallback)
	}
}

func returnsPromise(ctx *engine.Context, n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		if wrapperchecker.IsAsyncFunction(n) {
			return true
		}
		t := ctx.TypeOf(n)
		if t == nil {
			return false
		}
		for _, sig := range t.CallSignatures() {
			rt := sig.ReturnType()
			if rt == nil { continue }
			if rt.IsPromise() { return true }
			if rt.IsUnion() {
				for _, m := range rt.UnionMembers() {
					if m.IsPromise() { return true }
				}
			}
		}
	}
	return false
}

func allCallSignaturesExpectVoid(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		anyVoidCallable := false
		for _, m := range t.UnionMembers() {
			// Skip nullable members — `() => void | undefined` still
			// requires a void-returning callable on the value side.
			if m.IsNullOrUndefined() {
				continue
			}
			if !allCallSignaturesExpectVoid(m) {
				return false
			}
			anyVoidCallable = true
		}
		return anyVoidCallable
	}
	sigs := t.CallSignatures()
	if len(sigs) == 0 {
		return false
	}
	for _, s := range sigs {
		ret := s.ReturnType()
		if ret == nil {
			return false
		}
		if !ret.IsVoid() {
			return false
		}
	}
	return true
}

func (r *rule) visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	cond := conditionExpression(n)
	if cond == nil {
		return
	}
	checkPromiseInTest(ctx, cond)
}

func (r *rule) visitForStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	cond := n.ForStatementCondition()
	if cond == nil {
		return
	}
	checkPromiseInTest(ctx, cond)
}

func (r *rule) visitBinaryExpression(ctx *engine.Context, n *wrapperchecker.Node) {
	// Catch assignments to void-returning function-typed targets.
	if r.opts.ChecksVoidReturn && n.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
		left := n.BinaryLeft()
		right := n.BinaryRight()
		if left == nil || right == nil {
			return
		}
		if !returnsPromise(ctx, right) {
			return
		}
		t := ctx.TypeOf(left)
		if t != nil && allCallSignaturesExpectVoid(t) {
			ctx.Report(right, msgVoidCallback)
		}
	}
}

func (r *rule) visitVariableDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	if !returnsPromise(ctx, init) {
		return
	}
	annot := n.VariableDeclarationType()
	if annot == nil {
		return
	}
	t := ctx.Checker().TypeFromTypeNode(annot)
	if t == nil {
		return
	}
	if allCallSignaturesExpectVoid(t) {
		ctx.Report(init, msgVoidCallback)
	}
}

func (r *rule) visitShorthandProperty(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	// In `{ f }`, the value expression is the same identifier `f` —
	// the contextual type is what the surrounding object type expects.
	id := n.FirstChild()
	if id == nil {
		return
	}
	t := ctx.TypeOf(id)
	if t == nil {
		return
	}
	if !returnTypeIsPromise(t) {
		return
	}
	expected := ctx.Checker().ContextualTypeOf(id)
	if expected == nil {
		return
	}
	if allCallSignaturesExpectVoid(expected) {
		ctx.Report(n, msgVoidCallback)
	}
}

func (r *rule) visitMethodSignature(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	// Method signature on an interface — check return-type annotation.
	ann := n.FunctionReturnTypeAnnotation()
	if ann == nil {
		return
	}
	rt := ctx.Checker().TypeFromTypeNode(ann)
	if rt == nil || (!rt.IsPromise() && !returnTypeIsPromiseUnion(rt)) {
		return
	}
	parent := n.Parent()
	if parent == nil || parent.Kind() != wrapperchecker.KindInterfaceDeclaration {
		return
	}
	name := methodName(n)
	if name == "" {
		return
	}
	for _, base := range parent.HeritageTypes(ctx.Checker()) {
		expected := base.PropertyType(name)
		if expected == nil {
			continue
		}
		if allCallSignaturesExpectVoid(expected) {
			ctx.Report(n, msgVoidCallback)
			return
		}
	}
}

func returnTypeIsPromiseUnion(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				return true
			}
		}
	}
	return false
}

func (r *rule) visitMethodDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	if !wrapperchecker.IsAsyncFunction(n) && !methodReturnsPromise(ctx, n) {
		return
	}
	parent := n.Parent()
	if parent == nil {
		return
	}
	switch parent.Kind() {
	case wrapperchecker.KindObjectLiteralExpression:
		// `{ async f() {} }` — contextual type from surrounding annotation.
		expected := ctx.Checker().ContextualTypeOf(n)
		if expected == nil {
			return
		}
		if allCallSignaturesExpectVoid(expected) {
			ctx.Report(n, msgVoidCallback)
		}
	case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
		// `class X implements I { async f() {} }` — check each heritage
		// type for a same-named property whose call signatures expect
		// void.
		methodName := methodName(n)
		if methodName == "" {
			return
		}
		for _, base := range parent.HeritageTypes(ctx.Checker()) {
			expected := base.PropertyType(methodName)
			if expected == nil {
				continue
			}
			if allCallSignaturesExpectVoid(expected) {
				ctx.Report(n, msgVoidCallback)
				return
			}
		}
	}
}

func methodName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func methodReturnsPromise(ctx *engine.Context, n *wrapperchecker.Node) bool {
	t := ctx.TypeOf(n)
	if t == nil {
		return false
	}
	return returnTypeIsPromise(t)
}

func returnTypeIsPromise(t *wrapperchecker.Type) bool {
	for _, sig := range t.CallSignatures() {
		rt := sig.ReturnType()
		if rt == nil {
			continue
		}
		if rt.IsPromise() {
			return true
		}
		if rt.IsUnion() {
			for _, m := range rt.UnionMembers() {
				if m.IsPromise() {
					return true
				}
			}
		}
	}
	return false
}

func (r *rule) visitPropertyAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	init := n.PropertyInitializer()
	if init == nil {
		return
	}
	if !returnsPromise(ctx, init) {
		return
	}
	expected := ctx.Checker().ContextualTypeOf(init)
	if expected == nil {
		return
	}
	if allCallSignaturesExpectVoid(expected) {
		ctx.Report(init, msgVoidCallback)
	}
}

func (r *rule) visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	checkPromiseInTest(ctx, operand)
}

func (r *rule) visitSpreadElement(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksSpreads {
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
	if t.IsPromise() {
		ctx.Report(n, msgSpread)
		return
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				ctx.Report(n, msgSpread)
				return
			}
		}
	}
}

func checkPromiseInTest(ctx *engine.Context, expr *wrapperchecker.Node) {
	// `a || b` and `a && b` evaluate either operand depending on a's
	// truthiness; testing the whole expression is testing each operand
	// in turn. Descend so a promise on one side is flagged even when
	// the other side is a non-promise. `??` doesn't have this property
	// (LHS is tested for null/undefined, not truthiness), so check the
	// whole expression type.
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindBarBarToken,
			wrapperchecker.KindAmpersandAmpersandToken:
			if l := expr.BinaryLeft(); l != nil {
				checkPromiseInTest(ctx, l)
			}
			if r := expr.BinaryRight(); r != nil {
				checkPromiseInTest(ctx, r)
			}
			return
		}
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isTestPromise(t) {
		ctx.Report(expr, msgConditional)
	}
}

// isTestPromise reports whether testing this type's truthiness is
// almost certainly a misuse: the type is purely Promise, or a union
// where every non-nullable member is Promise (so the test fires only
// on a promise, never on a real value).
func isTestPromise(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		hasPromise := false
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			if m.IsPromise() {
				hasPromise = true
				continue
			}
			// Any non-nullable, non-promise member means the test could
			// legitimately be operating on a real value.
			return false
		}
		return hasPromise
	}
	return t.IsPromise()
}

func conditionExpression(n *wrapperchecker.Node) *wrapperchecker.Node {
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
