// Package noawaitinloop implements the no-await-in-loop rule: an
// `await` inside a loop body serialises asynchronous work that could
// often be parallelised with Promise.all. The rule walks up from each
// await (or `for await...of`, or `await using` declaration) to its
// nearest non-function ancestor; if that ancestor is a loop and the
// await sits in its body / test / update — not its pre-loop init — we
// flag it.
package noawaitinloop

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-await-in-loop"

const message = "Unexpected `await` inside a loop."

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAwaitExpression:        r.visit,
		wrapperchecker.KindForOfStatement:         r.visit,
		wrapperchecker.KindVariableDeclarationList: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !triggers(n) {
		return
	}
	cur := n
	parent := n.Parent()
	for parent != nil && !isFunctionBoundary(parent) {
		// `for await...of` is a boundary unless WE are the for-await node.
		if isForAwait(parent) && !nodesEqual(parent, n) {
			return
		}
		if isLoopedThrough(cur, parent, n) {
			ctx.Report(n, message)
			return
		}
		cur = parent
		parent = parent.Parent()
	}
}

// triggers reports whether n is one of the three node shapes that the
// rule treats as an in-loop offence: a plain `await`, the `await` of a
// `for await...of` header, or an `await using` declaration list.
func triggers(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindAwaitExpression:
		return true
	case wrapperchecker.KindForOfStatement:
		return n.HasAwaitModifier()
	case wrapperchecker.KindVariableDeclarationList:
		return n.IsAwaitUsingDeclaration()
	}
	return false
}

// isFunctionBoundary identifies the ancestor kinds that stop the
// upward walk because a fresh async context begins inside them.
func isFunctionBoundary(n *wrapperchecker.Node) bool {
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

func isForAwait(n *wrapperchecker.Node) bool {
	return n.Kind() == wrapperchecker.KindForOfStatement && n.HasAwaitModifier()
}

// isLoopedThrough reports whether `cur` (a direct child of `parent`)
// sits in a part of `parent` that re-runs each iteration. `origin` is
// the original node we are walking up from; it matters only for the
// `await using` in for-in/for-of left position.
func isLoopedThrough(cur, parent, origin *wrapperchecker.Node) bool {
	switch parent.Kind() {
	case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement:
		return nodesEqual(cur, parent.IterationBody()) || nodesEqual(cur, parent.WhileCondition())
	case wrapperchecker.KindForStatement:
		return nodesEqual(cur, parent.IterationBody()) ||
			nodesEqual(cur, parent.ForStatementCondition()) ||
			nodesEqual(cur, parent.ForStatementIncrementor())
	case wrapperchecker.KindForInStatement, wrapperchecker.KindForOfStatement:
		if nodesEqual(cur, parent.IterationBody()) {
			return true
		}
		// `for (await using x of xs) {}` — the left position is rebound
		// every iteration, so it counts as inside the loop.
		if origin.Kind() == wrapperchecker.KindVariableDeclarationList &&
			origin.IsAwaitUsingDeclaration() &&
			nodesEqual(cur, parent.ForInOrOfInitializer()) {
			return true
		}
	}
	return false
}

// nodesEqual compares two AST nodes by source position. Two distinct
// AST nodes cannot share both Pos and End, so this is sufficient for
// identity checks while we walk parent chains.
func nodesEqual(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Pos() == b.Pos() && a.End() == b.End()
}
