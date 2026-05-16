// Package nothisbeforesuper implements the no-this-before-super rule.
// In a derived class constructor, every `this` or `super` reference
// must follow a `super(...)` call on every possible execution path;
// touching `this` first raises ReferenceError at runtime. We walk each
// derived-class constructor body once, tracking whether super has been
// called in the current flow, and report uses that happen too early.
//
// This is a conservative port of oxc's CFG-based analysis: we do
// per-block flow propagation for if/try/finally explicitly and treat
// loops as "may not execute," which matches every fixture upstream
// ships.
package nothisbeforesuper

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-this-before-super"

const message = "Expected to always call `super()` before `this`/`super` property access."

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: r.visit,
		wrapperchecker.KindClassExpression:  r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !extendsNonNull(n) {
		return
	}
	ctor := findConstructor(n)
	if ctor == nil {
		return
	}
	body := ctor.FunctionBody()
	if body == nil {
		return
	}
	st := &flowState{}
	walkBlock(ctx, body, st)
}

// extendsNonNull reports whether the class has an `extends X` clause
// where X is not the literal `null`. Without an extends or with
// `extends null`, super() is not required and this/super refs are
// fine.
func extendsNonNull(class *wrapperchecker.Node) bool {
	found := false
	class.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindHeritageClause {
			return false
		}
		if c.HeritageClauseToken() != wrapperchecker.KindExtendsKeyword {
			return false
		}
		c.ForEachChild(func(expr *wrapperchecker.Node) bool {
			ex := expr
			if ex.Kind() == wrapperchecker.KindExpressionWithTypeArguments {
				ex = ex.ExpressionWithTypeArgumentsExpression()
			}
			if ex != nil && ex.Kind() != wrapperchecker.KindNullKeyword {
				found = true
			}
			return false
		})
		return false
	})
	return found
}

// findConstructor returns the first KindConstructor member of the
// class, or nil if none exists.
func findConstructor(class *wrapperchecker.Node) *wrapperchecker.Node {
	var ctor *wrapperchecker.Node
	class.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindConstructor {
			ctor = c
		}
		return false
	})
	return ctor
}

// flowState tracks whether super() has been called along the current
// path. It is intentionally minimal — branches clone the state, then
// the parent merges via AND.
type flowState struct {
	superCalled bool
	terminated  bool
}

func (s *flowState) clone() *flowState { c := *s; return &c }

// walkBlock walks the statements of a Block in order.
func walkBlock(ctx *engine.Context, block *wrapperchecker.Node, st *flowState) {
	if block.Kind() != wrapperchecker.KindBlock {
		walkStatement(ctx, block, st)
		return
	}
	for _, s := range block.BlockStatements() {
		if st.terminated {
			return
		}
		walkStatement(ctx, s, st)
	}
}

// walkStatement dispatches by statement kind. It mutates st to
// reflect any super() call that definitely ran in straight-line flow,
// and recurses through branches with cloned state.
func walkStatement(ctx *engine.Context, n *wrapperchecker.Node, st *flowState) {
	switch n.Kind() {
	case wrapperchecker.KindBlock:
		walkBlock(ctx, n, st)
	case wrapperchecker.KindIfStatement:
		walkExpression(ctx, n.IfCondition(), st)
		thenState := st.clone()
		walkStatement(ctx, n.IfThen(), thenState)
		var elseState *flowState
		if e := n.IfElse(); e != nil {
			elseState = st.clone()
			walkStatement(ctx, e, elseState)
		}
		if elseState != nil {
			if thenState.superCalled && elseState.superCalled {
				st.superCalled = true
			}
		}
	case wrapperchecker.KindTryStatement:
		tryState := st.clone()
		if b := n.TryStatementTryBlock(); b != nil {
			walkBlock(ctx, b, tryState)
		}
		catch := n.TryStatementCatchClause()
		if catch != nil {
			catchState := st.clone()
			walkCatchClause(ctx, catch, catchState)
		}
		// Finally always runs; analyse it with the pre-try state
		// because an exception can occur before super in the try.
		finallyState := st.clone()
		if f := n.TryStatementFinallyBlock(); f != nil {
			walkBlock(ctx, f, finallyState)
		}
		// If the try block ran to completion super was called; with
		// no catch we can propagate. With a catch the catch path may
		// have skipped super so we conservatively do not propagate.
		if catch == nil && tryState.superCalled {
			st.superCalled = true
		}
	case wrapperchecker.KindWhileStatement,
		wrapperchecker.KindDoStatement,
		wrapperchecker.KindForStatement,
		wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement:
		// Walk the body with a cloned state so we still report
		// violations inside the loop, but loop completion does not
		// imply super was called.
		loopState := st.clone()
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walkStatement(ctx, c, loopState)
			return false
		})
	case wrapperchecker.KindSwitchStatement:
		// Conservative: treat each case as may-or-may-not-execute.
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			cs := st.clone()
			walkStatement(ctx, c, cs)
			return false
		})
	case wrapperchecker.KindReturnStatement,
		wrapperchecker.KindThrowStatement:
		walkExpressionAll(ctx, n, st)
		st.terminated = true
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration:
		// Nested function/class declarations have their own `this`
		// binding — do not descend.
	default:
		walkExpressionAll(ctx, n, st)
	}
}

// isShortCircuit reports whether `op` is a binary operator whose
// right-hand operand may be skipped: `&&`, `||`, `??`, and their
// logical-assignment variants.
func isShortCircuit(op wrapperchecker.Kind) bool {
	switch op {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken,
		wrapperchecker.KindQuestionQuestionToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

// walkCatchClause walks the catch clause's body.
func walkCatchClause(ctx *engine.Context, catch *wrapperchecker.Node, st *flowState) {
	catch.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			walkBlock(ctx, c, st)
		}
		return false
	})
}

// walkExpressionAll walks every expression in the subtree of n, but
// does not cross statement boundaries. Used for expression
// statements and standalone expressions where every nested
// occurrence of this/super matters.
func walkExpressionAll(ctx *engine.Context, n *wrapperchecker.Node, st *flowState) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		walkExpression(ctx, c, st)
		return false
	})
}

// walkExpression descends through an expression in evaluation order.
// We approximate evaluation order by ForEachChild's source order,
// which is accurate enough for the patterns the rule cares about
// (operators, calls, property accesses, parenthesized exprs).
func walkExpression(ctx *engine.Context, n *wrapperchecker.Node, st *flowState) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindMethodDeclaration:
		// Inner function-like / class scopes get their own `this`.
		return
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		if isShortCircuit(op) {
			// Left-hand side runs unconditionally; right-hand side is
			// guarded by the short-circuit. Walk the rhs with a
			// cloned state so super-called there does not propagate.
			walkExpression(ctx, n.BinaryLeft(), st)
			rhsState := st.clone()
			walkExpression(ctx, n.BinaryRight(), rhsState)
			return
		}
		walkExpression(ctx, n.BinaryLeft(), st)
		walkExpression(ctx, n.BinaryRight(), st)
		return
	case wrapperchecker.KindCallExpression:
		callee := n.CalleeExpression()
		if callee != nil && callee.Kind() == wrapperchecker.KindSuperKeyword {
			// Walk args first — `super(this.x)` is a violation
			// because this is read before super() completes.
			for _, a := range n.CallArguments() {
				walkExpression(ctx, a, st)
			}
			st.superCalled = true
			return
		}
		walkExpression(ctx, callee, st)
		for _, a := range n.CallArguments() {
			walkExpression(ctx, a, st)
		}
		return
	case wrapperchecker.KindThisKeyword:
		if !st.superCalled {
			ctx.Report(n, message)
		}
		return
	case wrapperchecker.KindSuperKeyword:
		// A bare super (super.foo, super[x]) without a preceding
		// super() is a violation. super() calls are handled above.
		if !st.superCalled {
			ctx.Report(n, message)
		}
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		walkExpression(ctx, c, st)
		return false
	})
}
