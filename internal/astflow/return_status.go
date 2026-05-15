// Package astflow provides AST-level approximations of code-flow
// analysis: "does every path through this body return a value?",
// "does every path call super()?", etc. These are the building blocks
// for rules like array-callback-return, getter-return,
// constructor-super, no-fallthrough, and no-unreachable-loop.
//
// The analysis is intentionally syntactic — it walks the AST and
// merges branch outcomes for if/switch/try. It does not consult the
// type checker or build a real CFG, so a few pathological shapes
// (computed `return` exits via labeled break, e.g.) may be
// misclassified. The classifications match ESLint's own behavior on
// the cases that matter for the rules built on top.
package astflow

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
)

// ReturnStatus classifies how a function body terminates. Every leaf
// path is either an explicit `return value;` / `throw;`, an implicit
// fall-off-the-end, or "always-explicit/implicit/mixed" depending on
// how the branches combine. Matches oxc's StatementReturnStatus
// shape.
type ReturnStatus int

const (
	// NotReturn means no path through the body returns; control just
	// runs off the end.
	NotReturn ReturnStatus = iota
	// AlwaysExplicit means every path ends in `return value;` or
	// `throw`.
	AlwaysExplicit
	// AlwaysImplicit means every path ends in a value-less `return;`
	// or runs off the end.
	AlwaysImplicit
	// AlwaysMixed means every path returns but some are explicit and
	// some implicit — typically `if (...) return x; ` with the else
	// branch missing.
	AlwaysMixed
	// SomeExplicit means at least one path returns a value, but not
	// every path returns.
	SomeExplicit
	// SomeImplicit means at least one path returns without a value,
	// but not every path returns.
	SomeImplicit
	// SomeMixed means paths exist for both explicit and implicit
	// return, plus paths that don't return at all.
	SomeMixed
)

// MustReturn reports whether every path through the body terminates
// in a return or throw.
func (s ReturnStatus) MustReturn() bool {
	return s == AlwaysExplicit || s == AlwaysImplicit || s == AlwaysMixed
}

// MayReturnExplicit reports whether some path returns a value.
func (s ReturnStatus) MayReturnExplicit() bool {
	switch s {
	case AlwaysExplicit, AlwaysMixed, SomeExplicit, SomeMixed:
		return true
	}
	return false
}

// MayReturnImplicit reports whether some path returns without a
// value or runs off the end.
func (s ReturnStatus) MayReturnImplicit() bool {
	switch s {
	case AlwaysImplicit, AlwaysMixed, SomeImplicit, SomeMixed, NotReturn:
		return true
	}
	return false
}

// FunctionBodyReturnStatus classifies the body of a function-like
// node (Function, FunctionExpression, ArrowFunction, MethodDeclaration,
// GetAccessor, SetAccessor, Constructor). Returns NotReturn for nodes
// that have no body (an abstract declaration) or whose body is an
// expression (concise-body arrow `() => x` is always AlwaysExplicit;
// `async () => undefined` is also AlwaysExplicit because async
// functions always wrap their return value in a Promise).
func FunctionBodyReturnStatus(fn *wrapperchecker.Node) ReturnStatus {
	if fn == nil {
		return NotReturn
	}
	body, isExprBody := functionBody(fn)
	if body == nil {
		return NotReturn
	}
	// Concise-body arrow: `() => expr`. The expression is the return
	// value, full stop — always explicit.
	if isExprBody {
		return AlwaysExplicit
	}
	return analyzeBlock(body)
}

// functionBody returns the body node of a function-like declaration
// and a flag indicating whether the body is an expression
// (concise-body arrow). For block-bodied functions, isExprBody is
// false. Returns nil, false when the function has no body.
func functionBody(fn *wrapperchecker.Node) (body *wrapperchecker.Node, isExprBody bool) {
	switch fn.Kind() {
	case wrapperchecker.KindArrowFunction:
		// Arrow body is the last child; it's either a Block or an
		// expression.
		var last *wrapperchecker.Node
		fn.ForEachChild(func(c *wrapperchecker.Node) bool {
			last = c
			return false
		})
		if last == nil {
			return nil, false
		}
		if last.Kind() == wrapperchecker.KindBlock {
			return last, false
		}
		return last, true
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindConstructor:
		var block *wrapperchecker.Node
		fn.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindBlock {
				block = c
				return true
			}
			return false
		})
		return block, false
	}
	return nil, false
}

// analyzeBlock walks a Block, IfStatement, SwitchStatement, or any
// statement-shaped node and returns its ReturnStatus. The recursive
// helpers below handle each statement form.
func analyzeBlock(n *wrapperchecker.Node) ReturnStatus {
	if n == nil {
		return NotReturn
	}
	switch n.Kind() {
	case wrapperchecker.KindBlock:
		return analyzeStatements(blockStatements(n))
	case wrapperchecker.KindIfStatement:
		return analyzeIf(n)
	case wrapperchecker.KindSwitchStatement:
		return analyzeSwitch(n)
	case wrapperchecker.KindTryStatement:
		return analyzeTry(n)
	case wrapperchecker.KindReturnStatement:
		if returnHasValue(n) {
			return AlwaysExplicit
		}
		return AlwaysImplicit
	case wrapperchecker.KindThrowStatement:
		// Throw counts as "explicit exit" — a path that doesn't
		// fall through to an implicit return.
		return AlwaysExplicit
	case wrapperchecker.KindForStatement,
		wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement,
		wrapperchecker.KindWhileStatement,
		wrapperchecker.KindDoStatement:
		// Loops may execute zero times; treat them as never
		// guaranteeing a return. Inner returns still contribute to
		// "may return explicit/implicit".
		inner := loopBody(n)
		s := analyzeBlock(inner)
		return demoteToSome(s)
	}
	// Plain statements (expression, declaration, etc.) don't return.
	return NotReturn
}

// analyzeStatements walks a block's statements left to right. A
// terminating statement (return/throw, or a child statement whose
// status is MustReturn) cuts the walk short — subsequent statements
// are unreachable. The running aggregate tracks must-return + any
// explicit-value/explicit-void observations. Fall-off-end of a child
// statement does NOT count as an "implicit" observation — only
// explicit bare `return;` does.
func analyzeStatements(stmts []*wrapperchecker.Node) ReturnStatus {
	var mayExplicit, mayImplicit bool
	for _, stmt := range stmts {
		s := analyzeBlock(stmt)
		if s.MayReturnExplicit() {
			mayExplicit = true
		}
		if hasExplicitVoidReturn(s) {
			mayImplicit = true
		}
		if s.MustReturn() {
			return makeStatus(true, mayExplicit, mayImplicit)
		}
	}
	return makeStatus(false, mayExplicit, mayImplicit)
}

// analyzeIf merges the then/else branches. Without an else, the
// missing branch is an implicit fall-through.
func analyzeIf(n *wrapperchecker.Node) ReturnStatus {
	var cond, thenBranch, elseBranch *wrapperchecker.Node
	count := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch count {
		case 0:
			cond = c
		case 1:
			thenBranch = c
		case 2:
			elseBranch = c
		}
		count++
		return false
	})
	_ = cond
	thenS := analyzeBlock(thenBranch)
	if elseBranch == nil {
		// No else: fallthrough is implicit-not-return.
		return mergeBranches(thenS, NotReturn)
	}
	elseS := analyzeBlock(elseBranch)
	return mergeBranches(thenS, elseS)
}

// analyzeSwitch composes the outcome of each clause as an entry point.
// A clause that doesn't terminate falls through to the next, so we
// walk right-to-left and let each clause's status absorb the
// subsequent clauses' status. The per-clause results are then merged
// as branches (any clause may match). When no `default` is present, an
// extra "no match" branch contributes a fall-through path.
func analyzeSwitch(n *wrapperchecker.Node) ReturnStatus {
	var clauses []*wrapperchecker.Node
	hasDefault := false
	// SwitchStatement's case clauses live one level down, inside the
	// CaseBlock node. Recursively descend through any non-clause
	// children so we don't depend on a CaseBlock kind constant.
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
		return NotReturn
	}
	// Walk right-to-left. `next` is the status that falls out of the
	// remaining clauses; each iteration computes the entry-point status
	// for the current clause by composing its own body with `next` if
	// it doesn't terminate.
	statuses := make([]ReturnStatus, len(clauses))
	next := NotReturn // off the end of the switch
	for i := len(clauses) - 1; i >= 0; i-- {
		body := analyzeStatements(clauseStatements(clauses[i]))
		entry := body
		if !body.MustReturn() {
			entry = mergeFallthrough(body, next)
		}
		statuses[i] = entry
		next = entry
	}
	result := statuses[0]
	for i := 1; i < len(statuses); i++ {
		result = mergeBranches(result, statuses[i])
	}
	if !hasDefault {
		// No matching case: control falls off the switch entirely.
		result = mergeBranches(result, NotReturn)
	}
	return result
}

// analyzeTry merges try/catch/finally. A finally that always exits
// dominates the result; otherwise, try and catch outcomes merge.
func analyzeTry(n *wrapperchecker.Node) ReturnStatus {
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
	tryS := analyzeBlock(tryB)
	catchS := NotReturn
	if catchB != nil {
		var catchBlock *wrapperchecker.Node
		catchB.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindBlock {
				catchBlock = c
				return true
			}
			return false
		})
		catchS = analyzeBlock(catchBlock)
	}
	finallyS := NotReturn
	if finallyB != nil {
		finallyS = analyzeBlock(finallyB)
	}
	if finallyS.MustReturn() {
		return finallyS
	}
	if catchB != nil {
		return mergeBranches(tryS, catchS)
	}
	// No catch: any throw in try escapes; treat the try outcome as
	// the merged status (caller decides what to do with un-caught
	// throws).
	return tryS
}

// mergeBranches combines two branch outcomes into one. Both must
// return for the result to be Always; otherwise the result is Some.
// "Implicit" tracks only observed bare `return;` — a missing else
// branch contributes fall-off, not an implicit return.
func mergeBranches(a, b ReturnStatus) ReturnStatus {
	must := a.MustReturn() && b.MustReturn()
	mayExplicit := a.MayReturnExplicit() || b.MayReturnExplicit()
	mayImplicit := hasExplicitVoidReturn(a) || hasExplicitVoidReturn(b)
	return makeStatus(must, mayExplicit, mayImplicit)
}

// mergeFallthrough is mergeBranches biased for switch case
// fallthrough: prev's observed returns contribute, and cur's
// terminator (or lack thereof) determines the combined must-return.
func mergeFallthrough(prev, cur ReturnStatus) ReturnStatus {
	mayExplicit := prev.MayReturnExplicit() || cur.MayReturnExplicit()
	mayImplicit := hasExplicitVoidReturn(prev) || hasExplicitVoidReturn(cur)
	return makeStatus(cur.MustReturn(), mayExplicit, mayImplicit)
}

// hasExplicitVoidReturn reports whether a status reflects at least one
// observed bare `return;` — i.e., excludes the "ran off the end"
// case. Used internally to keep fall-off-end paths distinct from
// explicit-void returns when merging branches.
func hasExplicitVoidReturn(s ReturnStatus) bool {
	switch s {
	case AlwaysImplicit, AlwaysMixed, SomeImplicit, SomeMixed:
		return true
	}
	return false
}

// demoteToSome converts an Always* status to its Some* counterpart
// — used when wrapping a guaranteed-return body in a context that
// might skip it (a loop that may iterate zero times).
func demoteToSome(s ReturnStatus) ReturnStatus {
	switch s {
	case AlwaysExplicit:
		return SomeExplicit
	case AlwaysImplicit:
		return SomeImplicit
	case AlwaysMixed:
		return SomeMixed
	}
	return s
}

func makeStatus(must, explicit, implicit bool) ReturnStatus {
	switch {
	case must && explicit && implicit:
		return AlwaysMixed
	case must && explicit:
		return AlwaysExplicit
	case must && implicit:
		return AlwaysImplicit
	case explicit && implicit:
		return SomeMixed
	case explicit:
		return SomeExplicit
	case implicit:
		return SomeImplicit
	}
	return NotReturn
}

// blockStatements returns the direct children of a Block.
func blockStatements(block *wrapperchecker.Node) []*wrapperchecker.Node {
	if block == nil {
		return nil
	}
	var out []*wrapperchecker.Node
	block.ForEachChild(func(c *wrapperchecker.Node) bool {
		out = append(out, c)
		return false
	})
	return out
}

// clauseStatements returns the statement list of a case/default
// clause. The clause's first child is the case expression (for
// CaseClause); subsequent children are the statements.
func clauseStatements(clause *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	saw := 0
	clause.ForEachChild(func(c *wrapperchecker.Node) bool {
		saw++
		if clause.Kind() == wrapperchecker.KindCaseClause && saw == 1 {
			// Skip the case expression.
			return false
		}
		out = append(out, c)
		return false
	})
	return out
}

// returnHasValue reports whether a ReturnStatement has an
// argument expression (`return x;` vs `return;`).
func returnHasValue(ret *wrapperchecker.Node) bool {
	has := false
	ret.ForEachChild(func(c *wrapperchecker.Node) bool {
		has = true
		return true
	})
	return has
}

// loopBody returns the body block/statement of a loop.
func loopBody(loop *wrapperchecker.Node) *wrapperchecker.Node {
	var last *wrapperchecker.Node
	loop.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

