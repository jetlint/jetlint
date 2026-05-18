// Package useiterablecallbackreturn implements biome's
// `useIterableCallbackReturn` rule (id `use-iterable-callback-return`).
//
// Non-forEach methods reuse the [arraycallbackreturn] implementation
// (biome and oxlint agree there). The forEach branch is checked
// locally with biome's looser semantics: only an explicit
// `return <non-void expr>` (or a concise-body arrow whose body is a
// non-void expression) is flagged. Bare returns, void expressions,
// throw-only paths, and empty bodies are accepted — biome treats them
// as intentional discards.
//
// biome defaults `checkForEach` to true (the rule's whole point);
// passing `checkForEach: false` falls back to the eslint behaviour of
// only checking value-consuming methods.
package useiterablecallbackreturn

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/arraycallbackreturn"
)

const id = "use-iterable-callback-return"

// Options mirrors [arraycallbackreturn.Options]; only the defaults
// differ.
type Options = arraycallbackreturn.Options

// DefaultOptions returns biome's defaults: checkForEach on.
func DefaultOptions() Options { return Options{CheckForEach: true} }

// OptionsFromJSON parses biome-shaped options.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("use-iterable-callback-return options must be a JSON object: %w", err)
	}
	for k, v := range fields {
		switch k {
		case "allowImplicit":
			if err := json.Unmarshal(v, &out.AllowImplicit); err != nil {
				return Options{}, err
			}
		case "checkForEach":
			if err := json.Unmarshal(v, &out.CheckForEach); err != nil {
				return Options{}, err
			}
		case "allowVoid":
			if err := json.Unmarshal(v, &out.AllowVoid); err != nil {
				return Options{}, err
			}
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

// NewWithOptions wraps arraycallbackreturn for non-forEach methods
// (with its own forEach branch disabled) and overlays the local
// forEach check.
func NewWithOptions(opts Options) engine.Rule {
	innerOpts := opts
	innerOpts.CheckForEach = false
	return &rule{
		opts:  opts,
		inner: arraycallbackreturn.NewWithOptions(innerOpts),
	}
}

type rule struct {
	opts  Options
	inner engine.Rule
}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	innerHandlers := r.inner.Handlers()
	innerCall := innerHandlers[wrapperchecker.KindCallExpression]
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: func(ctx *engine.Context, n *wrapperchecker.Node) {
			if innerCall != nil {
				innerCall(ctx, n)
			}
			if r.opts.CheckForEach {
				r.checkForEach(ctx, n)
			}
		},
	}
}

// checkForEach flags `forEach` callbacks that explicitly return a
// non-void value. The detection mirrors arraycallbackreturn's
// `arrayMethodName` for `forEach` only (any property/element access
// whose name is `forEach`).
func (r *rule) checkForEach(ctx *engine.Context, call *wrapperchecker.Node) {
	if !isForEachCall(call) {
		return
	}
	args := call.CallArguments()
	if len(args) == 0 {
		return
	}
	cb := stripParens(args[0])
	if cb == nil || !isFunctionLike(cb) {
		return
	}
	if wrapperchecker.IsAsyncFunction(cb) || cb.IsGeneratorFunction() {
		return
	}
	if conciseBody, isConcise := arrowConciseBody(cb); isConcise {
		if !isVoidExpression(conciseBody) {
			ctx.Report(cb, "Array.forEach callback should not return a value")
		}
		return
	}
	if hasExplicitNonVoidReturn(functionBlockBody(cb)) {
		ctx.Report(cb, "Array.forEach callback should not return a value")
	}
}

func isForEachCall(call *wrapperchecker.Node) bool {
	callee := stripParens(call.CalleeExpression())
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName() == "forEach"
	case wrapperchecker.KindElementAccessExpression:
		idx := elementIndex(callee)
		if idx == nil {
			return false
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText() == "forEach"
		}
	}
	return false
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		return true
	}
	return false
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

func elementIndex(n *wrapperchecker.Node) *wrapperchecker.Node {
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

// arrowConciseBody returns the body expression of a concise-body
// arrow (and true). For block-bodied arrows or non-arrow functions,
// returns (nil, false).
func arrowConciseBody(fn *wrapperchecker.Node) (*wrapperchecker.Node, bool) {
	if fn.Kind() != wrapperchecker.KindArrowFunction {
		return nil, false
	}
	var last *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	if last == nil || last.Kind() == wrapperchecker.KindBlock {
		return nil, false
	}
	return last, true
}

// functionBlockBody returns the Block body of a function-like node,
// or nil if the function has no block body (concise-body arrow,
// abstract).
func functionBlockBody(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var block *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			block = c
			return true
		}
		return false
	})
	return block
}

// isVoidExpression reports whether n (after paren stripping) is a
// `void <expr>` expression.
func isVoidExpression(n *wrapperchecker.Node) bool {
	n = stripParens(n)
	return n != nil && n.Kind() == wrapperchecker.KindVoidExpression
}

// hasExplicitNonVoidReturn reports whether the given block contains
// an explicit `return <expr>` statement whose <expr> is not a void
// expression. Nested function-likes are skipped (their returns
// belong to their own body). `return;` (bare) is ignored.
func hasExplicitNonVoidReturn(block *wrapperchecker.Node) bool {
	if block == nil {
		return false
	}
	found := false
	var visit func(n *wrapperchecker.Node)
	visit = func(n *wrapperchecker.Node) {
		if n == nil || found {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return
		case wrapperchecker.KindReturnStatement:
			var arg *wrapperchecker.Node
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				arg = c
				return true
			})
			if arg != nil && !isVoidExpression(arg) {
				found = true
			}
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			visit(c)
			return false
		})
	}
	visit(block)
	return found
}
