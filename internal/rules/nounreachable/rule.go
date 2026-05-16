// Package nounreachable implements the no-unreachable rule: code
// that lives after a `return`, `throw`, `break`, or `continue` (or an
// equivalent always-exits construct like `if (x) return; else throw`)
// can never run. We approximate oxc's CFG analysis with an explicit
// AST-level "this statement always exits" check, then walk each block
// and flag the first unreachable statement.
package nounreachable

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unreachable"

const message = "Unreachable code."

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBlock:         r.visitBlockLike,
		wrapperchecker.KindSourceFile:    r.visitBlockLike,
		wrapperchecker.KindModuleBlock:   r.visitBlockLike,
		wrapperchecker.KindCaseClause:    r.visitCaseClause,
		wrapperchecker.KindDefaultClause: r.visitCaseClause,
	}
}

func (r *rule) visitBlockLike(ctx *engine.Context, n *wrapperchecker.Node) {
	stmts := blockStatements(n)
	r.scanStatements(ctx, stmts)
}

func (r *rule) visitCaseClause(ctx *engine.Context, n *wrapperchecker.Node) {
	var stmts []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		// Skip the case-expression child of a CaseClause; only
		// collect statement children.
		if isStatementKind(c.Kind()) {
			stmts = append(stmts, c)
		}
		return false
	})
	r.scanStatements(ctx, stmts)
}

func (r *rule) scanStatements(ctx *engine.Context, stmts []*wrapperchecker.Node) {
	terminated := false
	for _, s := range stmts {
		if terminated && !isHoistedDeclaration(s) {
			ctx.Report(s, message)
			return
		}
		if statementAlwaysExits(s) {
			terminated = true
		}
	}
}

// blockStatements returns the statement list of a Block, SourceFile,
// or ModuleBlock node.
func blockStatements(n *wrapperchecker.Node) []*wrapperchecker.Node {
	if n.Kind() == wrapperchecker.KindBlock {
		return n.BlockStatements()
	}
	var out []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if isStatementKind(c.Kind()) {
			out = append(out, c)
		}
		return false
	})
	return out
}

// isStatementKind reports whether `k` is a statement node we care
// about for reachability scanning. We exclude purely declarative
// nodes that are hoisted (handled via isHoistedDeclaration below).
func isStatementKind(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindBlock,
		wrapperchecker.KindVariableStatement,
		wrapperchecker.KindExpressionStatement,
		wrapperchecker.KindIfStatement,
		wrapperchecker.KindDoStatement,
		wrapperchecker.KindWhileStatement,
		wrapperchecker.KindForStatement,
		wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement,
		wrapperchecker.KindContinueStatement,
		wrapperchecker.KindBreakStatement,
		wrapperchecker.KindReturnStatement,
		wrapperchecker.KindSwitchStatement,
		wrapperchecker.KindLabeledStatement,
		wrapperchecker.KindThrowStatement,
		wrapperchecker.KindTryStatement,
		wrapperchecker.KindDebuggerStatement,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration:
		return true
	}
	return false
}

// isHoistedDeclaration reports whether `s` is a declaration whose
// presence does not represent "running code" at the position it
// appears — function declarations are hoisted in full, and `var`
// declarations without initializers are hoisted as bindings.
func isHoistedDeclaration(s *wrapperchecker.Node) bool {
	switch s.Kind() {
	case wrapperchecker.KindFunctionDeclaration:
		return true
	case wrapperchecker.KindVariableStatement:
		declList := s.VariableStatementDeclarationList()
		if declList == nil {
			return true
		}
		// `var` declarations whose declarators all lack an
		// initializer are hoisted. `let`/`const` or any declarator
		// with an initializer counts as real, reachable code.
		if !isVarDeclaration(declList) {
			return false
		}
		anyInit := false
		declList.ForEachChild(func(d *wrapperchecker.Node) bool {
			if init := d.VariableDeclarationInitializer(); init != nil {
				anyInit = true
			}
			return false
		})
		return !anyInit
	}
	return false
}

// isVarDeclaration reports whether the declaration list uses `var`
// (as opposed to `let` or `const`). The wrapper exposes this via the
// Const flag; we treat anything that isn't a let or const as `var`.
func isVarDeclaration(declList *wrapperchecker.Node) bool {
	// `var` declarations have neither Let nor Const flags; the easy
	// distinguishing signal is the source-text prefix.
	text := declList.SourceText()
	return len(text) >= 4 && text[:4] == "var "
}

// statementAlwaysExits reports whether `n` always transfers control
// out of the enclosing block on every execution path.
func statementAlwaysExits(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindReturnStatement,
		wrapperchecker.KindThrowStatement,
		wrapperchecker.KindBreakStatement,
		wrapperchecker.KindContinueStatement:
		return true
	case wrapperchecker.KindBlock:
		for _, s := range n.BlockStatements() {
			if statementAlwaysExits(s) {
				return true
			}
		}
		return false
	case wrapperchecker.KindIfStatement:
		then := n.IfThen()
		els := n.IfElse()
		if els == nil {
			return false
		}
		return statementAlwaysExits(then) && statementAlwaysExits(els)
	case wrapperchecker.KindTryStatement:
		tryBlock := n.TryStatementTryBlock()
		catch := n.TryStatementCatchClause()
		finally := n.TryStatementFinallyBlock()
		if finally != nil && statementAlwaysExits(finally) {
			return true
		}
		tryExits := tryBlock != nil && statementAlwaysExits(tryBlock)
		if catch == nil {
			return tryExits
		}
		var catchBody *wrapperchecker.Node
		catch.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindBlock {
				catchBody = c
			}
			return false
		})
		catchExits := catchBody != nil && statementAlwaysExits(catchBody)
		return tryExits && catchExits
	case wrapperchecker.KindWhileStatement:
		if isLiteralTrue(n.WhileCondition()) && !containsBreak(n.IterationBody()) {
			return true
		}
		return false
	case wrapperchecker.KindDoStatement:
		if body := n.IterationBody(); body != nil && statementAlwaysExits(body) {
			return true
		}
		if cond := getDoStatementCondition(n); isLiteralTrue(cond) && !containsBreak(n.IterationBody()) {
			return true
		}
		return false
	case wrapperchecker.KindForStatement:
		// Infinite `for (;;)` or `for (;true;)` with no break.
		cond := n.ForStatementCondition()
		if (cond == nil || isLiteralTrue(cond)) && !containsBreak(n.IterationBody()) {
			return true
		}
		return false
	case wrapperchecker.KindLabeledStatement:
		// A labeled statement is targetable by `break <label>` /
		// `continue <label>`, which exits the labeled block but
		// leaves control with the surrounding scope. Conservatively
		// treat a labeled statement as not always exiting; the
		// uncommon case where its body return/throws still produces
		// no false positives downstream.
		return false
	case wrapperchecker.KindSwitchStatement:
		// Conservative: a switch always exits only if every case
		// (including a default) always exits without a break-out.
		// This case is rarely the source of "reachable below"
		// situations in the wild; default to "does not exit".
		return false
	}
	return false
}

func getDoStatementCondition(n *wrapperchecker.Node) *wrapperchecker.Node {
	var cond *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		// In a do-while, ForEachChild emits the body (statement)
		// first and then the condition expression.
		if !isStatementKind(c.Kind()) {
			cond = c
		}
		return false
	})
	return cond
}

func isLiteralTrue(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindTrueKeyword
}

// containsBreak walks the subtree looking for an unlabeled `break`
// that would target the enclosing loop. We do not descend into
// nested loops or switches, since their `break` is captured locally.
func containsBreak(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindBreakStatement:
			found = true
			return
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindForStatement,
			wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement,
			wrapperchecker.KindSwitchStatement:
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
