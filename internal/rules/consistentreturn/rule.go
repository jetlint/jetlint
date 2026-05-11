// Package consistentreturn implements the consistent-return rule:
// flag functions whose return statements mix `return;` (no value) and
// `return X` (with value) — except where the declared return type is
// `void` and the inconsistency is intentional.
package consistentreturn

import (
	"encoding/json"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "consistent-return"

// Options mirrors the upstream eslint consistent-return options.
// `TreatUndefinedAsUnspecified`, when true, collapses returns whose
// value is the `undefined` literal or whose type is `undefined` into
// the bare-return bucket — so `if(x) return undefined; return;` is
// considered consistent.
type Options struct {
	TreatUndefinedAsUnspecified bool
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 {
		return out, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return out, err
	}
	if v, ok := m["treatUndefinedAsUnspecified"]; ok {
		if err := json.Unmarshal(v, &out.TreatUndefinedAsUnspecified); err != nil {
			return out, err
		}
	}
	return out, nil
}

func New() engine.Rule                        { return &rule{} }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: r.visit,
		wrapperchecker.KindFunctionExpression:  r.visit,
		wrapperchecker.KindArrowFunction:       r.visit,
		wrapperchecker.KindMethodDeclaration:   r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	body := n.FunctionBody()
	if body == nil || body.Kind() != wrapperchecker.KindBlock {
		return
	}
	hasValue, hasUndefValue, hasBare := collectReturns(ctx, body, n)
	// `treatUndefinedAsUnspecified` makes `return undefined` semantically
	// equivalent to `return;`. With it on, only mixing a non-undefined
	// value with a bare/undefined return is inconsistent.
	if r.opts.TreatUndefinedAsUnspecified {
		if !hasValue || !(hasBare || hasUndefValue) {
			return
		}
	} else {
		// Default: a typed-undefined return counts as a value return,
		// so two value paths are consistent even if one is undefined-
		// typed; only mixing any value with bare `return;` flags.
		if !hasBare || !(hasValue || hasUndefValue) {
			return
		}
	}
	// `(): void` explicitly opts into ignoring the value of any
	// returns; the rule's job is satisfied by the user's annotation.
	if returnTypeIsVoid(ctx, n) {
		return
	}
	ctx.Report(n, "function returns values inconsistently — some paths return a value, others use a bare `return;`")
}

func collectReturns(ctx *engine.Context, body, fn *wrapperchecker.Node) (hasValue, hasUndefValue, hasBare bool) {
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
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
			switch {
			case expr == nil:
				hasBare = true
			case returnsUndefinedOrVoid(ctx, expr):
				hasUndefValue = true
			default:
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
		// Async: unwrap Promise<X> recursively. `Promise<ReturnType<typeof
		// asyncFn>>` resolves to `Promise<Promise<void>>` and the body's
		// awaited value is `void`, so peel off all Promise layers.
		return typeContainsVoid(unwrapPromise(t))
	}
	return typeContainsVoid(t)
}

// unwrapPromise repeatedly strips `Promise<X>` from a type. Walks up
// to a few levels to handle `Promise<ReturnType<typeof asyncFn>>` →
// `Promise<Promise<void>>` → `void`.
func unwrapPromise(t *wrapperchecker.Type) *wrapperchecker.Type {
	for i := 0; i < 8 && t != nil; i++ {
		isPromise := t.SymbolName() == "Promise"
		if !isPromise {
			for _, base := range t.BaseTypeNames() {
				if base == "Promise" {
					isPromise = true
					break
				}
			}
		}
		if !isPromise {
			return t
		}
		args := t.TypeArguments()
		if len(args) == 0 {
			return t
		}
		t = args[0]
	}
	return t
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
