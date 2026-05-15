package astflow

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
)

// SuperCallStatus describes how a derived constructor's body
// satisfies (or fails to satisfy) the rule that every path through
// the body reaches `super()` exactly once. Paths that exit via
// `throw` or `return <value>` are exempt — they bypass normal object
// construction.
type SuperCallStatus int

const (
	// SuperNone: at least one path neither calls super nor exits
	// cleanly; no path called super at all.
	SuperNone SuperCallStatus = iota
	// SuperAlways: every path either called super exactly once or
	// exited cleanly via throw/return-value.
	SuperAlways
	// SuperSome: at least one path called super and at least one
	// did not (and the latter didn't exit cleanly either).
	SuperSome
	// SuperMultiple: at least one path called super more than once.
	SuperMultiple
)

// pathState tracks super-call distribution and exit categorization
// across the paths through a node:
//
//   - aliveSuper: super count distribution on paths still executing
//     (haven't exited via throw/return/break).
//   - aliveExists: at least one alive path exists.
//   - bareExitSuper: super count distribution on paths that have
//     bare-`return;`-exited so far.
//   - bareExitExists: at least one bare-exit path exists.
//   - cleanExitExists: at least one path exited via throw or
//     `return <value>` (informational; those paths are exempt).
//   - broke: every alive path was terminated by top-of-clause
//     break/continue (used by switch-clause analysis).
type pathState struct {
	aliveSuper      SuperCallStatus
	aliveExists     bool
	bareExitSuper   SuperCallStatus
	bareExitExists  bool
	cleanExitExists bool
	broke           bool
}

// startState is the entry pathState: one alive path with no super
// calls yet.
var startState = pathState{aliveExists: true}

// ConstructorHasSuperCall reports whether any `super(...)` call
// syntactically appears in the constructor's body (outside nested
// function/class declarations). Used by the invalid-super cases (no
// extends, extends null, extends literal) which just need an
// existence check.
func ConstructorHasSuperCall(ctor *wrapperchecker.Node) bool {
	if ctor == nil {
		return false
	}
	var body *wrapperchecker.Node
	ctor.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
			return true
		}
		return false
	})
	if body == nil {
		return false
	}
	return superInExpr(body) != SuperNone
}

// ConstructorSuperCallStatus returns the SuperCallStatus of a
// Constructor's body.
func ConstructorSuperCallStatus(ctor *wrapperchecker.Node) SuperCallStatus {
	if ctor == nil {
		return SuperNone
	}
	var body *wrapperchecker.Node
	ctor.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
			return true
		}
		return false
	})
	if body == nil {
		return SuperNone
	}
	final := composeSequential(startState, analyzeStmtSeq(blockStatements(body)))
	return finalize(final)
}

// finalize converts the final pathState into the rule-facing
// SuperCallStatus.
func finalize(s pathState) SuperCallStatus {
	// Multiple super calls anywhere is an error regardless of
	// satisfaction.
	if s.aliveSuper == SuperMultiple || s.bareExitSuper == SuperMultiple {
		return SuperMultiple
	}
	// Compute the joint super status over "non-clean" paths (those
	// that need super to be considered satisfied).
	var nonClean SuperCallStatus
	hasNonClean := false
	if s.aliveExists {
		nonClean = s.aliveSuper
		hasNonClean = true
	}
	if s.bareExitExists {
		if hasNonClean {
			nonClean = mergeSuperBranches(nonClean, s.bareExitSuper)
		} else {
			nonClean = s.bareExitSuper
			hasNonClean = true
		}
	}
	if !hasNonClean {
		// Every path cleanly exited — no super required.
		return SuperAlways
	}
	return nonClean
}

// analyzeNode computes the pathState delta for a node. The delta
// describes what happens to a single "entry" alive path passing
// through this node.
func analyzeNode(n *wrapperchecker.Node) pathState {
	if n == nil {
		return passThrough()
	}
	switch n.Kind() {
	case wrapperchecker.KindBlock:
		return analyzeStmtSeq(blockStatements(n))
	case wrapperchecker.KindIfStatement:
		return analyzeIfNode(n)
	case wrapperchecker.KindSwitchStatement:
		return analyzeSwitchNode(n)
	case wrapperchecker.KindTryStatement:
		return analyzeTryNode(n)
	case wrapperchecker.KindReturnStatement:
		var arg *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			arg = c
			return true
		})
		// Run any super calls in the return argument before exiting.
		out := stepSuperOnAlive(passThrough(), superInExpr(arg))
		// Now exit: alive → clean or bare.
		return exitAlivePaths(out, arg != nil)
	case wrapperchecker.KindThrowStatement:
		var arg *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			arg = c
			return true
		})
		out := stepSuperOnAlive(passThrough(), superInExpr(arg))
		// Throw is always a clean exit.
		return exitAlivePaths(out, true)
	case wrapperchecker.KindBreakStatement,
		wrapperchecker.KindContinueStatement:
		return pathState{broke: true}
	case wrapperchecker.KindForStatement,
		wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement,
		wrapperchecker.KindWhileStatement,
		wrapperchecker.KindDoStatement:
		inner := analyzeNode(loopBody(n))
		// Inner return/throw exits propagate out of the loop — they
		// exit the whole function, not just the iteration. Only one
		// iteration ever bare-exits (the rest don't run).
		out := passThrough()
		if inner.bareExitExists {
			out.bareExitExists = true
			out.bareExitSuper = inner.bareExitSuper
		}
		if inner.cleanExitExists {
			out.cleanExitExists = true
		}
		// Alive post-loop: super count is the SUM across however many
		// iterations completed (0..N). If inner.aliveSuper is None,
		// post-loop alive super is None. Otherwise mark Multiple to
		// capture the possible repetition (the rule treats Multiple
		// as an error, matching oxlint's CFG-based pessimism).
		if inner.aliveSuper != SuperNone {
			out.aliveSuper = SuperMultiple
		}
		return out
	case wrapperchecker.KindLabeledStatement:
		return analyzeNode(labeledBody(n))
	case wrapperchecker.KindExpressionStatement:
		var arg *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			arg = c
			return true
		})
		return stepSuperOnAlive(passThrough(), superInExpr(arg))
	case wrapperchecker.KindVariableStatement:
		out := passThrough()
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			out = stepSuperOnAlive(out, superInExpr(c))
			return false
		})
		return out
	}
	return stepSuperOnAlive(passThrough(), superInExpr(n))
}

// passThrough is the identity delta: an entry alive path stays alive
// with no super calls added.
func passThrough() pathState {
	return pathState{aliveSuper: SuperNone, aliveExists: true}
}

// stepSuperOnAlive applies super calls to all alive paths.
func stepSuperOnAlive(s pathState, calls SuperCallStatus) pathState {
	if calls == SuperNone {
		return s
	}
	if !s.aliveExists {
		return s
	}
	s.aliveSuper = composeSuperSequential(s.aliveSuper, calls)
	return s
}

// exitAlivePaths converts alive paths to exits. When cleanExit is
// true the path exited via throw/return-value; otherwise via bare
// `return;`.
func exitAlivePaths(s pathState, cleanExit bool) pathState {
	if !s.aliveExists {
		return s
	}
	if cleanExit {
		s.cleanExitExists = true
	} else {
		// Bare exit: record super count at exit time.
		if s.bareExitExists {
			s.bareExitSuper = mergeSuperBranches(s.bareExitSuper, s.aliveSuper)
		} else {
			s.bareExitSuper = s.aliveSuper
			s.bareExitExists = true
		}
	}
	s.aliveSuper = SuperNone
	s.aliveExists = false
	return s
}

// analyzeStmtSeq composes a sequence of statements left-to-right.
func analyzeStmtSeq(stmts []*wrapperchecker.Node) pathState {
	acc := passThrough()
	for _, stmt := range stmts {
		d := analyzeNode(stmt)
		acc = composeSequential(acc, d)
		if acc.broke || !acc.aliveExists {
			return acc
		}
	}
	return acc
}

// analyzeIfNode merges then/else with the condition.
func analyzeIfNode(n *wrapperchecker.Node) pathState {
	var cond, thenBranch, elseBranch *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch idx {
		case 0:
			cond = c
		case 1:
			thenBranch = c
		case 2:
			elseBranch = c
		}
		idx++
		return false
	})
	condDelta := stepSuperOnAlive(passThrough(), superInExpr(cond))
	thenDelta := analyzeNode(thenBranch)
	var elseDelta pathState
	if elseBranch != nil {
		elseDelta = analyzeNode(elseBranch)
	} else {
		elseDelta = passThrough()
	}
	branched := mergeBranches2(thenDelta, elseDelta)
	return composeSequential(condDelta, branched)
}

// analyzeSwitchNode composes clauses with fall-through.
func analyzeSwitchNode(n *wrapperchecker.Node) pathState {
	var clauses []*wrapperchecker.Node
	hasDefault := false
	var visit func(*wrapperchecker.Node)
	visit = func(p *wrapperchecker.Node) {
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindCaseClause:
				clauses = append(clauses, c)
			case wrapperchecker.KindDefaultClause:
				clauses = append(clauses, c)
				hasDefault = true
			default:
				visit(c)
			}
			return false
		})
	}
	visit(n)
	if len(clauses) == 0 {
		return passThrough()
	}
	deltas := make([]pathState, len(clauses))
	next := passThrough()
	for i := len(clauses) - 1; i >= 0; i-- {
		body := analyzeStmtSeq(clauseStatements(clauses[i]))
		var entry pathState
		switch {
		case body.broke:
			// Break exits the switch. Clear broke; alive paths
			// continue past the switch.
			entry = body
			entry.broke = false
			entry.aliveExists = true
			entry.aliveSuper = body.aliveSuper
		case !body.aliveExists:
			// Body terminated via return/throw — no fallthrough.
			entry = body
		default:
			// Body falls through to the next clause.
			entry = composeSequential(body, next)
		}
		deltas[i] = entry
		next = entry
	}
	result := deltas[0]
	for i := 1; i < len(deltas); i++ {
		result = mergeBranches2(result, deltas[i])
	}
	if !hasDefault {
		// No matching case → control falls through unchanged.
		result = mergeBranches2(result, passThrough())
	}
	return result
}

// analyzeTryNode handles try/catch/finally.
func analyzeTryNode(n *wrapperchecker.Node) pathState {
	var tryB, catchB, finallyB *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindBlock:
			if tryB == nil {
				tryB = c
			} else if finallyB == nil {
				finallyB = c
			}
		case wrapperchecker.KindCatchClause:
			catchB = c
		}
		return false
	})
	tryS := analyzeNode(tryB)
	catchS := pathState{}
	if catchB != nil {
		var catchBlock *wrapperchecker.Node
		catchB.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindBlock {
				catchBlock = c
				return true
			}
			return false
		})
		catchS = analyzeNode(catchBlock)
	} else {
		catchS = passThrough()
	}
	body := mergeBranches2(tryS, catchS)
	if finallyB != nil {
		return composeSequential(body, analyzeNode(finallyB))
	}
	return body
}

// composeSequential composes a delta after a prior state. The
// resulting state covers the same set of paths as a, with the alive
// paths from a now also experiencing b's effects.
func composeSequential(a, b pathState) pathState {
	if a.broke {
		return a
	}
	if !a.aliveExists {
		// No alive paths in a — b runs on nothing. Carry through.
		return a
	}
	// b describes what happens to ONE alive entry path. Since a has
	// alive paths (possibly with various super counts), we need to
	// compose b's effect with a's alive super distribution.
	out := pathState{
		cleanExitExists: a.cleanExitExists || b.cleanExitExists,
		bareExitExists:  a.bareExitExists,
		bareExitSuper:   a.bareExitSuper,
		broke:           b.broke,
	}
	// Apply b's super count to a's alive paths' super count.
	aliveSuperAfter := composeSuperSequential(a.aliveSuper, b.aliveSuper)
	out.aliveSuper = aliveSuperAfter
	out.aliveExists = b.aliveExists
	// If b caused some paths to bare-exit, those paths take the
	// post-step super count. b's bareExitSuper is relative to b's
	// entry; we need to combine with a's prior super on those paths.
	if b.bareExitExists {
		// Paths that bare-exited inside b carried (a.aliveSuper + b's
		// bareExit contribution before exit).
		shifted := composeSuperSequential(a.aliveSuper, b.bareExitSuper)
		if out.bareExitExists {
			out.bareExitSuper = mergeSuperBranches(out.bareExitSuper, shifted)
		} else {
			out.bareExitSuper = shifted
			out.bareExitExists = true
		}
	}
	return out
}

// mergeBranches2 merges two alternative pathStates (then/else,
// switch clauses, try/catch).
func mergeBranches2(a, b pathState) pathState {
	out := pathState{
		cleanExitExists: a.cleanExitExists || b.cleanExitExists,
		broke:           a.broke && b.broke,
	}
	// Alive paths in the merge: union of alive paths from each branch.
	switch {
	case a.aliveExists && b.aliveExists:
		out.aliveExists = true
		out.aliveSuper = mergeSuperBranches(a.aliveSuper, b.aliveSuper)
	case a.aliveExists:
		out.aliveExists = true
		out.aliveSuper = a.aliveSuper
	case b.aliveExists:
		out.aliveExists = true
		out.aliveSuper = b.aliveSuper
	}
	// Bare-exit paths.
	switch {
	case a.bareExitExists && b.bareExitExists:
		out.bareExitExists = true
		out.bareExitSuper = mergeSuperBranches(a.bareExitSuper, b.bareExitSuper)
	case a.bareExitExists:
		out.bareExitExists = true
		out.bareExitSuper = a.bareExitSuper
	case b.bareExitExists:
		out.bareExitExists = true
		out.bareExitSuper = b.bareExitSuper
	}
	return out
}

// superInExpr scans an expression subtree for direct super(...) calls.
func superInExpr(n *wrapperchecker.Node) SuperCallStatus {
	if n == nil {
		return SuperNone
	}
	switch n.Kind() {
	case wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression:
		return SuperNone
	case wrapperchecker.KindCallExpression:
		callee := stripExprParens(n.CalleeExpression())
		if callee != nil && callee.Kind() == wrapperchecker.KindSuperKeyword {
			argSuper := SuperNone
			for _, a := range n.CallArguments() {
				argSuper = composeSuperSequential(argSuper, superInExpr(a))
			}
			return composeSuperSequential(argSuper, SuperAlways)
		}
		var acc SuperCallStatus
		acc = composeSuperSequential(acc, superInExpr(callee))
		for _, a := range n.CallArguments() {
			acc = composeSuperSequential(acc, superInExpr(a))
		}
		return acc
	case wrapperchecker.KindConditionalExpression:
		whenTrue, whenFalse := n.ConditionalBranches()
		cond := n.ConditionalCondition()
		condCall := superInExpr(cond)
		branchCall := mergeSuperBranches(
			superInExpr(whenTrue),
			superInExpr(whenFalse),
		)
		return composeSuperSequential(condCall, branchCall)
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		if op == wrapperchecker.KindAmpersandAmpersandToken ||
			op == wrapperchecker.KindBarBarToken ||
			op == wrapperchecker.KindQuestionQuestionToken {
			left := superInExpr(n.BinaryLeft())
			right := superInExpr(n.BinaryRight())
			return composeSuperSequential(left, mergeSuperBranches(right, SuperNone))
		}
	}
	var acc SuperCallStatus
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		acc = composeSuperSequential(acc, superInExpr(c))
		return false
	})
	return acc
}

// composeSuperSequential combines two super counts observed on the
// same path (in order).
func composeSuperSequential(a, b SuperCallStatus) SuperCallStatus {
	if a == SuperNone {
		return b
	}
	if b == SuperNone {
		return a
	}
	return SuperMultiple
}

// mergeSuperBranches merges alternative paths' super statuses.
func mergeSuperBranches(a, b SuperCallStatus) SuperCallStatus {
	if a == b {
		return a
	}
	if a == SuperMultiple || b == SuperMultiple {
		return SuperMultiple
	}
	return SuperSome
}

// stripExprParens unwraps parenthesized expressions.
func stripExprParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
