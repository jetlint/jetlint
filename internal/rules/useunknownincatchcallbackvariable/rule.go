// Package useunknownincatchcallbackvariable implements the
// use-unknown-in-catch-callback-variable rule: in `.catch(cb)` and
// `.then(_, cb)`, the callback's error parameter should be typed as
// `unknown` so callers must narrow before use.
package useunknownincatchcallbackvariable

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-unknown-in-catch-callback-variable"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	method, recv := methodCallName(ctx, callee)
	if method == "" {
		return
	}
	args := n.CallArguments()
	if hasSpreadArg(args) {
		// `Promise.resolve().then(...arr, cb)` — arg positions are
		// dynamic; can't reliably identify the catch slot.
		return
	}
	switch method {
	case "catch":
		if len(args) >= 1 {
			checkCatchCallback(ctx, recv, args[0])
		}
	case "then":
		if len(args) >= 2 {
			checkCatchCallback(ctx, recv, args[1])
		}
	}
}

// methodCallName returns the method-name string and receiver node for
// either a `recv.method` or `recv['method']`-style call. Empty name
// when the callee isn't a recognizable member access.
func methodCallName(ctx *engine.Context, callee *wrapperchecker.Node) (string, *wrapperchecker.Node) {
	if callee == nil {
		return "", nil
	}
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName(), callee.PropertyAccessReceiver()
	case wrapperchecker.KindElementAccessExpression:
		idx := callee.ElementAccessIndex()
		if idx == nil {
			return "", nil
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText(), callee.ElementAccessReceiver()
		}
		// `let method = 'catch'; obj[method](...)` — narrowing usually
		// gives the literal type; otherwise walk the symbol's variable
		// declaration to read the initializer's string value.
		if t := ctx.TypeOf(idx); t != nil {
			if v, ok := t.StringLiteralValue(); ok {
				return v, callee.ElementAccessReceiver()
			}
		}
		if v, ok := staticStringFromIdentifier(ctx, idx); ok {
			return v, callee.ElementAccessReceiver()
		}
	}
	return "", nil
}

// staticStringFromIdentifier resolves an identifier reference to its
// declared string-literal initializer. Returns the literal value when
// the symbol declares a single VariableDeclaration whose initializer
// is a string literal — covering `let method = 'catch'` patterns that
// TypeScript widens to plain `string`.
func staticStringFromIdentifier(ctx *engine.Context, n *wrapperchecker.Node) (string, bool) {
	if n == nil || n.Kind() != wrapperchecker.KindIdentifier {
		return "", false
	}
	sym := ctx.Checker().SymbolOf(n)
	if sym == nil {
		return "", false
	}
	for _, decl := range sym.Declarations() {
		if decl == nil || decl.Kind() != wrapperchecker.KindVariableDeclaration {
			continue
		}
		init := decl.VariableDeclarationInitializer()
		if init == nil {
			continue
		}
		switch init.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return init.LiteralText(), true
		}
	}
	return "", false
}

func hasSpreadArg(args []*wrapperchecker.Node) bool {
	for _, a := range args {
		if a.Kind() == wrapperchecker.KindSpreadElement {
			return true
		}
	}
	return false
}

func checkCatchCallback(ctx *engine.Context, recv, cb *wrapperchecker.Node) {
	if recv == nil {
		return
	}
	leaves := collectCallbackLeaves(cb)
	if len(leaves) == 0 {
		return
	}
	for _, leaf := range leaves {
		checkCatchLeaf(ctx, recv, leaf)
	}
}

// collectCallbackLeaves walks a callback expression through parens,
// ternaries, and short-circuit operators, returning every concrete
// arrow/function expression that could end up as the runtime handler.
// For an `||`/`??` chain like `(a || b)`, both `a` and `b` may be the
// chosen handler; the rule must check each.
func collectCallbackLeaves(n *wrapperchecker.Node) []*wrapperchecker.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case wrapperchecker.KindParenthesizedExpression:
		return collectCallbackLeaves(n.FirstChild())
	case wrapperchecker.KindConditionalExpression:
		whenTrue, whenFalse := n.ConditionalBranches()
		out := collectCallbackLeaves(whenTrue)
		out = append(out, collectCallbackLeaves(whenFalse)...)
		return out
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		switch op {
		case wrapperchecker.KindBarBarToken,
			wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindQuestionQuestionToken:
			out := collectCallbackLeaves(n.BinaryLeft())
			out = append(out, collectCallbackLeaves(n.BinaryRight())...)
			return out
		case wrapperchecker.KindCommaToken:
			// Comma evaluates every operand but yields only the right.
			// Side-effect callbacks on the left aren't candidates for
			// the runtime handler.
			return collectCallbackLeaves(n.BinaryRight())
		}
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		return []*wrapperchecker.Node{n}
	}
	return nil
}

func checkCatchLeaf(ctx *engine.Context, recv, cb *wrapperchecker.Node) {
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !isPromiseLike(rt) {
		return
	}
	if cb.Kind() != wrapperchecker.KindArrowFunction &&
		cb.Kind() != wrapperchecker.KindFunctionExpression {
		return
	}
	param := firstParameter(cb)
	if param == nil {
		return
	}
	annot := param.ParameterTypeAnnotation()
	if annot == nil {
		ctx.Report(param, "use `unknown` for catch-callback parameters; the rejection value is untyped at runtime")
		return
	}
	t := ctx.Checker().TypeFromTypeNode(annot)
	if t == nil {
		return
	}
	if isAcceptableErrorType(t, param.IsRestParameter()) {
		return
	}
	ctx.Report(param, "use `unknown` for catch-callback parameters; the rejection value is untyped at runtime")
}

// isAcceptableErrorType reports whether the parameter's annotation is
// `unknown`-equivalent. Rest parameters carry a tuple/array type whose
// elements must all be unknown.
func isAcceptableErrorType(t *wrapperchecker.Type, isRest bool) bool {
	if t == nil {
		return false
	}
	if !isRest {
		return t.IsUnknown()
	}
	if t.IsTupleType() {
		args := t.TypeArguments()
		if len(args) == 0 {
			return true
		}
		// The catch callback receives a single value; for a rest tuple
		// only the first element corresponds to that rejection value.
		return args[0].IsUnknown()
	}
	if elem := t.ArrayElementType(); elem != nil {
		return elem.IsUnknown()
	}
	return false
}

func firstParameter(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			first = c
			return true
		}
		return false
	})
	return first
}

func isPromiseLike(t *wrapperchecker.Type) bool {
	if t.IsPromise() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				return true
			}
		}
	}
	if t.IsThenable() {
		return true
	}
	return false
}
