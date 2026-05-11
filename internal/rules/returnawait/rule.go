// Package returnawait implements the return-await rule: in async
// functions, `return await X` either must or must not appear at each
// return position depending on whether X is a Promise, whether the
// return is inside an error-handling context (try/catch/finally or in
// scope of a `using`/`await using` declaration), and the configured
// mode (`always`, `error-handling-correctness-only`, `in-try-catch`,
// `never`). Ported from typescript-eslint's `return-await` rule.
package returnawait

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "return-await"

// Mode is the rule's behavioral mode.
type Mode string

const (
	ModeAlways                       Mode = "always"
	ModeErrorHandlingCorrectnessOnly Mode = "error-handling-correctness-only"
	ModeInTryCatch                   Mode = "in-try-catch"
	ModeNever                        Mode = "never"
)

// Options is the configurable surface of the rule.
type Options struct {
	Mode Mode
}

// DefaultOptions matches typescript-eslint's default of `in-try-catch`.
func DefaultOptions() Options { return Options{Mode: ModeInTryCatch} }

// OptionsFromJSON parses a single rule option entry. The upstream
// schema is a single string: `["always" | "error-handling-correctness-only" | "in-try-catch" | "never"]`.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	opts := DefaultOptions()
	if len(raw) == 0 {
		return opts, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch Mode(s) {
		case ModeAlways, ModeErrorHandlingCorrectnessOnly, ModeInTryCatch, ModeNever:
			opts.Mode = Mode(s)
			return opts, nil
		}
		return opts, fmt.Errorf("return-await: unknown mode %q", s)
	}
	return opts, fmt.Errorf("return-await: invalid option JSON %s", string(raw))
}

func New() engine.Rule                        { return &rule{opts: DefaultOptions()} }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindReturnStatement: r.visitReturn,
		wrapperchecker.KindArrowFunction:   r.visitArrow,
	}
}

// awaitable is upstream's tri-state for "does this need an await?".
type awaitable int

const (
	awaitableNever awaitable = iota
	awaitableMay
	awaitableAlways
)

// whetherToAwait is upstream's tri-state for the rule's verdict in a
// given context.
type whetherToAwait int

const (
	verdictDontCare whetherToAwait = iota
	verdictAwait
	verdictNoAwait
)

func (r *rule) config() (errorHandling, ordinary whetherToAwait) {
	switch r.opts.Mode {
	case ModeAlways:
		return verdictAwait, verdictAwait
	case ModeErrorHandlingCorrectnessOnly:
		return verdictAwait, verdictDontCare
	case ModeNever:
		return verdictNoAwait, verdictNoAwait
	case ModeInTryCatch:
		return verdictAwait, verdictNoAwait
	default:
		return verdictAwait, verdictNoAwait
	}
}

func (r *rule) visitReturn(ctx *engine.Context, n *wrapperchecker.Node) {
	owner := containingFunction(n)
	if owner == nil || !wrapperchecker.HasAsyncModifier(owner) {
		return
	}
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	for _, child := range findPossiblyReturnedNodes(expr) {
		r.test(ctx, child)
	}
}

func (r *rule) visitArrow(ctx *engine.Context, n *wrapperchecker.Node) {
	if !wrapperchecker.HasAsyncModifier(n) {
		return
	}
	body := n.FunctionBody()
	if body == nil || body.Kind() == wrapperchecker.KindBlock {
		// Block-bodied arrows go through the ReturnStatement handler.
		return
	}
	for _, child := range findPossiblyReturnedNodes(body) {
		r.test(ctx, child)
	}
}

// findPossiblyReturnedNodes flattens ConditionalExpression branches so
// each possibly-returned value is checked independently — `cond ? a : b`
// has two return positions, not one. ParenthesizedExpression wrappers
// are unwrapped so `(cond ? a : b)` is treated identically.
func findPossiblyReturnedNodes(n *wrapperchecker.Node) []*wrapperchecker.Node {
	if n == nil {
		return nil
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		return findPossiblyReturnedNodes(n.FirstChild())
	}
	if n.Kind() == wrapperchecker.KindConditionalExpression {
		whenTrue := n.ConditionalWhenTrue()
		whenFalse := n.ConditionalWhenFalse()
		return append(
			append([]*wrapperchecker.Node(nil), findPossiblyReturnedNodes(whenFalse)...),
			findPossiblyReturnedNodes(whenTrue)...,
		)
	}
	return []*wrapperchecker.Node{n}
}

func (r *rule) test(ctx *engine.Context, expr *wrapperchecker.Node) {
	isAwait := expr.Kind() == wrapperchecker.KindAwaitExpression
	child := expr
	if isAwait {
		child = expr.FirstChild()
		if child == nil {
			return
		}
	}
	t := ctx.TypeOf(child)
	if t == nil {
		return
	}
	cert := needsToBeAwaited(t)
	if cert != awaitableAlways {
		if isAwait && cert == awaitableNever {
			ctx.Report(expr, "returning an awaited value that is not a promise is not allowed")
		}
		return
	}
	errorHandling, ordinary := r.config()
	shouldAwait := ordinary
	if affectsErrorHandling(expr) {
		shouldAwait = errorHandling
	}
	switch shouldAwait {
	case verdictAwait:
		if !isAwait {
			ctx.Report(expr, "returning an awaited promise is required in this context")
		}
	case verdictNoAwait:
		if isAwait {
			ctx.Report(expr, "returning an awaited promise is not allowed in this context")
		}
	}
}

// needsToBeAwaited mirrors upstream's tri-state classification.
func needsToBeAwaited(t *wrapperchecker.Type) awaitable {
	if t == nil {
		return awaitableNever
	}
	// Walk type-parameter constraints to the underlying constraint type.
	ct := t
	if ct.IsTypeParameter() {
		if c := ct.BaseConstraint(); c != nil && c != ct {
			ct = c
		} else {
			// Unconstrained generic — could be a Promise at runtime.
			return awaitableMay
		}
	}
	if ct.IsAny() || ct.IsUnknown() {
		return awaitableMay
	}
	if ct.IsPromise() || ct.IsThenable() {
		return awaitableAlways
	}
	return awaitableNever
}

// containingFunction returns the nearest enclosing function-like
// ancestor (FunctionDeclaration, FunctionExpression, ArrowFunction,
// MethodDeclaration), or nil if the node is not inside one.
func containingFunction(n *wrapperchecker.Node) *wrapperchecker.Node {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration:
			return p
		}
	}
	return nil
}

// affectsErrorHandling reports whether throwing at `node` would be
// observed by an enclosing try/catch/finally — directly or transitively
// — or by an in-scope `using` / `await using` declaration. A throw
// inside a try-block always affects error handling; one inside a
// catch-block does only when the try has a finally (or when an outer
// try wraps it); same for finally-blocks.
func affectsErrorHandling(n *wrapperchecker.Node) bool {
	if affectsExplicitErrorHandling(n) {
		return true
	}
	return affectsExplicitResourceManagement(n)
}

func affectsExplicitErrorHandling(n *wrapperchecker.Node) bool {
	child := n
	for ancestor := n.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		if isFunctionLike(ancestor) {
			return false
		}
		if ancestor.Kind() == wrapperchecker.KindTryStatement {
			block := classifyTryChild(ancestor, child)
			switch block {
			case "try":
				return true
			case "catch":
				if tryHasFinally(ancestor) {
					return true
				}
				return affectsExplicitErrorHandling(ancestor)
			case "finally":
				return affectsExplicitErrorHandling(ancestor)
			}
		}
		child = ancestor
	}
	return false
}

// classifyTryChild reports whether `child` is the try-block,
// catch-clause, or finally-block of `tryStmt`. Returns "" if `child` is
// none of those (e.g. an outer-context ancestor traversal mismatch).
func classifyTryChild(tryStmt, child *wrapperchecker.Node) string {
	if tb := tryStmt.TryStatementTryBlock(); tb != nil && nodeEquals(tb, child) {
		return "try"
	}
	if cc := tryStmt.TryStatementCatchClause(); cc != nil && nodeEquals(cc, child) {
		return "catch"
	}
	if fb := tryStmt.TryStatementFinallyBlock(); fb != nil && nodeEquals(fb, child) {
		return "finally"
	}
	return ""
}

func tryHasFinally(tryStmt *wrapperchecker.Node) bool {
	return tryStmt.TryStatementFinallyBlock() != nil
}

// affectsExplicitResourceManagement reports whether a `using` or
// `await using` declaration precedes the node within the same function
// scope. A `using` adds an implicit `try { ... } finally { dispose }`
// around remaining statements, so any throw past that line is
// effectively in a finally-affected context.
func affectsExplicitResourceManagement(n *wrapperchecker.Node) bool {
	for scope := n.Parent(); scope != nil; scope = scope.Parent() {
		// A using declaration affects only the block in which it
		// appears — when we cross a function boundary, stop.
		if isFunctionLike(scope) {
			return false
		}
		if scope.Kind() != wrapperchecker.KindBlock && scope.Kind() != wrapperchecker.KindSourceFile {
			continue
		}
		// Walk statements before our path and check for using-decls.
		nPos := n.Pos()
		found := false
		scope.ForEachChild(func(stmt *wrapperchecker.Node) bool {
			if stmt.End() >= nPos {
				return true // stop — we passed our node
			}
			if stmt.Kind() != wrapperchecker.KindVariableStatement {
				return false
			}
			dl := stmt.VariableStatementDeclarationList()
			if dl == nil {
				return false
			}
			if dl.IsUsingDeclaration() || dl.IsAwaitUsingDeclaration() {
				found = true
				return true
			}
			return false
		})
		if found {
			return true
		}
	}
	return false
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		return true
	}
	return false
}

// nodeEquals reports whether two *wrapperchecker.Node refer to the same
// underlying AST node. We can't compare the wrapper pointers directly
// (each accessor allocates a fresh wrapper), but their positions and
// kinds uniquely identify a syntactic node within a file.
func nodeEquals(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Kind() == b.Kind() && a.Pos() == b.Pos() && a.End() == b.End()
}
