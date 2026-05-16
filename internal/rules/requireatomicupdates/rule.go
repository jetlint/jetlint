// Package requireatomicupdates implements the require-atomic-updates
// rule: flag assignments of the form `x = <expr containing await x>`
// (or `x += await … x …`, etc.) inside async / generator functions.
// Between reading `x` and storing the new value, another async task
// can interleave and observe / overwrite `x`, producing a race.
//
// The check is purely syntactic and intentionally narrow:
//
//   - An assignment expression whose left-hand side is an identifier
//     `x`.
//   - The right-hand side contains an `AwaitExpression` or
//     `YieldExpression` AND an `Identifier` reference whose text is
//     `x`.
//
// This catches the canonical `x = x + await f()` shape that motivates
// the rule. Property updates (`obj.x = ...`) and the option to
// suppress within specific variable groupings are intentionally not
// modeled — the rule mirrors oxlint's minimal behavior and ESLint's
// most-common reported pattern.
package requireatomicupdates

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "require-atomic-updates"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isAssignmentOperator(n.BinaryOperatorKind()) {
		return
	}
	lhs := n.BinaryLeft()
	rhs := n.BinaryRight()
	if lhs == nil || rhs == nil || lhs.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	name := lhs.SourceText()
	if name == "" {
		return
	}
	if !rhsContainsAwaitOrYield(rhs) {
		return
	}
	if !rhsReadsIdentifier(rhs, name) {
		return
	}
	ctx.Report(n, "Possible race condition: '"+name+"' might be reassigned based on an outdated value.")
}

func isAssignmentOperator(op wrapperchecker.Kind) bool {
	switch op {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

func rhsContainsAwaitOrYield(n *wrapperchecker.Node) bool {
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindAwaitExpression, wrapperchecker.KindYieldExpression:
			found = true
			return
		case wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionDeclaration:
			// Nested function bodies have their own async boundary
			// and shouldn't drag the outer assignment into the race.
			return
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(n)
	return found
}

func rhsReadsIdentifier(n *wrapperchecker.Node, name string) bool {
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		if c.Kind() == wrapperchecker.KindPropertyAccessExpression {
			// Property access: only the receiver is a value read of
			// an identifier; recurse only into it, skip the property
			// name.
			walk(c.PropertyAccessReceiver())
			return
		}
		if c.Kind() == wrapperchecker.KindFunctionExpression ||
			c.Kind() == wrapperchecker.KindArrowFunction ||
			c.Kind() == wrapperchecker.KindFunctionDeclaration {
			return
		}
		if c.Kind() == wrapperchecker.KindIdentifier {
			if c.SourceText() == name {
				found = true
			}
			return
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(n)
	return found
}
