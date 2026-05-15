// Package arraycallbackreturn implements the array-callback-return
// rule: callbacks passed to Array methods that consume a return value
// (`map`, `filter`, `every`, `some`, `find`, `findIndex`, `findLast`,
// `findLastIndex`, `flatMap`, `reduce`, `reduceRight`, `sort`,
// `toSorted`) should explicitly return a value on every path.
//
// Missing returns in such callbacks silently leak `undefined`, which
// these methods then treat as a value of the underlying type — a
// common source of subtly-wrong results (e.g. `map` callback that
// only `console.log`s, producing `Array<undefined>`).
//
// `forEach` consumes no return value; with the `checkForEach` option,
// the rule flags callbacks that *do* return a value (catching code
// that meant to use `map` instead). Default is off.
//
// `allowImplicit: true` permits an explicit value-less `return;` to
// satisfy the rule. Default is off — only `return value;` and
// `throw` count.
//
// Behavior derived from oxc and ESLint; the body-flow analysis lives
// in [astflow.FunctionBodyReturnStatus].
package arraycallbackreturn

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/astflow"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "array-callback-return"

// Options is the configurable surface of array-callback-return.
type Options struct {
	AllowImplicit bool
	CheckForEach  bool
	AllowVoid     bool // permits `void expr` concise-body arrows under CheckForEach
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("array-callback-return options must be a JSON object: %w", err)
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

func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: r.visit,
	}
}

// targetMethods are the Array prototype methods whose callbacks must
// return a value. `forEach` is handled separately under
// `CheckForEach`.
var targetMethods = map[string]bool{
	"every":         true,
	"filter":        true,
	"find":          true,
	"findIndex":     true,
	"findLast":      true,
	"findLastIndex": true,
	"flatMap":       true,
	"map":           true,
	"reduce":        true,
	"reduceRight":   true,
	"some":          true,
	"sort":          true,
	"toSorted":      true,
}

func (r *rule) visit(ctx *engine.Context, call *wrapperchecker.Node) {
	method, isStatic, ok := arrayMethodName(call)
	if !ok {
		return
	}
	args := call.CallArguments()
	// For `Array.from(x, fn)` and `Array.fromAsync(x, fn)`, the
	// callback is the second argument; for instance methods, it's the
	// first.
	cbIdx := 0
	if isStatic {
		if method != "from" && method != "fromAsync" {
			return
		}
		if len(args) < 2 {
			return
		}
		cbIdx = 1
	} else if method == "forEach" {
		if r.opts.CheckForEach {
			r.checkForEachCallback(ctx, call)
		}
		return
	} else if !targetMethods[method] {
		return
	}
	if len(args) <= cbIdx {
		return
	}
	for _, cb := range resolveCallbacks(args[cbIdx]) {
		r.checkCallback(ctx, method, cb)
	}
}

// checkCallback runs the body-flow check against a single resolved
// function-like callback node.
func (r *rule) checkCallback(ctx *engine.Context, method string, cb *wrapperchecker.Node) {
	// Async functions always return a Promise — treat as explicit.
	if wrapperchecker.IsAsyncFunction(cb) {
		return
	}
	// Generator functions return an iterator — also always explicit.
	if cb.IsGeneratorFunction() {
		return
	}
	status := astflow.FunctionBodyReturnStatus(cb)
	if status == astflow.AlwaysExplicit {
		return
	}
	if status == astflow.AlwaysImplicit && r.opts.AllowImplicit {
		return
	}
	if status == astflow.AlwaysMixed && r.opts.AllowImplicit {
		return
	}
	ctx.Report(cb, "Array."+method+" callback must return a value on every path")
}

// checkForEachCallback flags callbacks to forEach that return a
// value — usually a leftover from refactoring map → forEach.
func (r *rule) checkForEachCallback(ctx *engine.Context, call *wrapperchecker.Node) {
	args := call.CallArguments()
	if len(args) == 0 {
		return
	}
	for _, cb := range resolveCallbacks(args[0]) {
		if wrapperchecker.IsAsyncFunction(cb) || cb.IsGeneratorFunction() {
			continue
		}
		if r.opts.AllowVoid && onlyReturnsVoid(cb) {
			continue
		}
		status := astflow.FunctionBodyReturnStatus(cb)
		if status.MayReturnExplicit() {
			ctx.Report(cb, "Array.forEach callback should not return a value")
		}
	}
}

// resolveCallbacks drills through parentheses, logical operators
// (`||`, `&&`, `??`), and ternaries to find every function-like
// expression that may end up as the callback at runtime. Each branch
// is checked independently — flag if any branch has a problematic
// callback.
func resolveCallbacks(n *wrapperchecker.Node) []*wrapperchecker.Node {
	n = stripParens(n)
	if n == nil {
		return nil
	}
	if isFunctionLike(n) {
		return []*wrapperchecker.Node{n}
	}
	switch n.Kind() {
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		if op == wrapperchecker.KindBarBarToken ||
			op == wrapperchecker.KindAmpersandAmpersandToken ||
			op == wrapperchecker.KindQuestionQuestionToken {
			return append(resolveCallbacks(n.BinaryLeft()), resolveCallbacks(n.BinaryRight())...)
		}
	case wrapperchecker.KindConditionalExpression:
		whenTrue, whenFalse := n.ConditionalBranches()
		return append(resolveCallbacks(whenTrue), resolveCallbacks(whenFalse)...)
	}
	return nil
}

// onlyReturnsVoid reports whether every explicit return in n returns a
// `void <expr>` expression (or n is a concise-body arrow whose body is
// `void <expr>`). Used by the AllowVoid option for `checkForEach` —
// callbacks that explicitly discard their value via `void` are
// considered intentional under that mode.
func onlyReturnsVoid(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindArrowFunction {
		// Concise-body arrow: body is the last child.
		var last *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			last = c
			return false
		})
		if last != nil && last.Kind() != wrapperchecker.KindBlock {
			return stripParens(last).Kind() == wrapperchecker.KindVoidExpression
		}
	}
	// Block-bodied: every explicit `return` must return a `void` expr.
	// If there are no explicit returns the callback already passes the
	// no-return check; treat as satisfied.
	hasAnyReturn := false
	allVoid := true
	var visit func(p *wrapperchecker.Node, isRoot bool)
	visit = func(p *wrapperchecker.Node, isRoot bool) {
		if p == nil {
			return
		}
		if !isRoot {
			// Don't recurse into nested function-likes — their returns
			// belong to *their* body.
			switch p.Kind() {
			case wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindMethodDeclaration:
				return
			}
		}
		if p.Kind() == wrapperchecker.KindReturnStatement {
			var arg *wrapperchecker.Node
			p.ForEachChild(func(c *wrapperchecker.Node) bool {
				arg = c
				return true
			})
			hasAnyReturn = true
			if arg == nil || stripParens(arg).Kind() != wrapperchecker.KindVoidExpression {
				allVoid = false
			}
			return
		}
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			visit(c, false)
			return false
		})
	}
	visit(n, true)
	return hasAnyReturn && allVoid
}

// arrayMethodName returns the method name being called and whether the
// receiver is the static `Array` identifier (e.g. `Array.from`). The
// callee must be a property or element access; plain function calls
// return ok=false.
func arrayMethodName(call *wrapperchecker.Node) (name string, isStatic bool, ok bool) {
	callee := stripParens(call.CalleeExpression())
	if callee == nil {
		return "", false, false
	}
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		name = callee.PropertyAccessName()
		isStatic = isArrayIdentifier(propertyAccessReceiver(callee))
		return name, isStatic, true
	case wrapperchecker.KindElementAccessExpression:
		idx := elementIndex(callee)
		if idx == nil {
			return "", false, false
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText(), false, true
		}
	}
	return "", false, false
}

// propertyAccessReceiver returns the `a` in `a.b`.
func propertyAccessReceiver(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		first = c
		return true
	})
	return first
}

// isArrayIdentifier reports whether n is the Identifier `Array` —
// the receiver of static methods like `Array.from`.
func isArrayIdentifier(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	return n.LiteralText() == "Array"
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression:
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
