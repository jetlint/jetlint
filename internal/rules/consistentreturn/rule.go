// Package consistentreturn implements the consistent-return rule:
// flag functions whose return statements mix `return;` (no value) and
// `return X` (with value) — except where the declared return type is
// `void` and the inconsistency is intentional.
package consistentreturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "consistent-return"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	body := n.FunctionBody()
	if body == nil || body.Kind() != wrapperchecker.KindBlock {
		return
	}
	hasValue, hasBare := collectReturns(ctx, body, n)
	if !(hasValue && hasBare) {
		return
	}
	// `(): void` explicitly opts into ignoring the value of any
	// returns; the rule's job is satisfied by the user's annotation.
	if returnTypeIsVoid(ctx, n) {
		return
	}
	ctx.Report(n, "function returns values inconsistently — some paths return a value, others use a bare `return;`")
}

func collectReturns(ctx *engine.Context, body, fn *wrapperchecker.Node) (hasValue, hasBare bool) {
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil || (hasValue && hasBare) {
			return
		}
		// Don't descend into nested function-likes — their returns
		// belong to those functions.
		if n != fn {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration:
				return
			}
		}
		if n.Kind() == wrapperchecker.KindReturnStatement {
			expr := n.FirstChild()
			if expr == nil {
				hasBare = true
			} else if returnsUndefinedOrVoid(ctx, expr) {
				// `return undefined`, `return void 0`, or returning a
				// call whose declared type is void — equivalent to a
				// bare `return;` for consistency-of-return purposes.
				hasBare = true
			} else {
				hasValue = true
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return
}

// returnsUndefinedOrVoid reports whether expr produces only the
// `undefined` or `void` type — in which case `return expr;` carries
// no information beyond a bare `return;`.
func returnsUndefinedOrVoid(ctx *engine.Context, expr *wrapperchecker.Node) bool {
	if expr == nil {
		return false
	}
	if expr.Kind() == wrapperchecker.KindVoidExpression {
		return true
	}
	if expr.Kind() == wrapperchecker.KindIdentifier && expr.LiteralText() == "undefined" {
		return true
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return false
	}
	if t.IsVoid() {
		return true
	}
	if t.String() == "undefined" {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			s := m.String()
			if s != "undefined" && s != "void" {
				return false
			}
		}
		return true
	}
	return false
}

// returnTypeIsVoid reports whether the function's declared return
// type explicitly contains `void`. For an async function, the
// resolved-value type is what's relevant — `Promise<void>` is
// equivalent to `void` for return-statement purposes.
func returnTypeIsVoid(ctx *engine.Context, fn *wrapperchecker.Node) bool {
	annot := fn.FunctionReturnTypeAnnotation()
	if annot == nil {
		return false
	}
	t := ctx.Checker().TypeFromTypeNode(annot)
	if t == nil {
		return false
	}
	if wrapperchecker.HasAsyncModifier(fn) {
		// Async: unwrap Promise<X> and check X.
		if t.SymbolName() == "Promise" {
			args := t.TypeArguments()
			if len(args) > 0 {
				return typeContainsVoid(args[0])
			}
		}
		for _, base := range t.BaseTypeNames() {
			if base == "Promise" {
				args := t.TypeArguments()
				if len(args) > 0 {
					return typeContainsVoid(args[0])
				}
			}
		}
	}
	return typeContainsVoid(t)
}

func typeContainsVoid(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsVoid() || t.String() == "void" {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeContainsVoid(m) {
				return true
			}
		}
	}
	return false
}
