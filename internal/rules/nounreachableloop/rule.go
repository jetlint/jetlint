// Package nounreachableloop implements the no-unreachable-loop rule:
// flag a loop whose body cannot iterate more than once because every
// path through it exits the loop (via `break`, `return`, or `throw`).
// `continue` is *not* an exit — it restarts the iteration.
//
// The analysis is purely syntactic. For each branching construct
// (`if`/`else`, `switch`, `try`) the rule checks that every branch
// unconditionally exits; for a sequence of statements, any statement
// that exits collapses the rest as unreachable.
package nounreachableloop

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unreachable-loop"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindWhileStatement:   r.visit,
		wrapperchecker.KindDoStatement:      r.visit,
		wrapperchecker.KindForStatement:     r.visit,
		wrapperchecker.KindForInStatement:   r.visit,
		wrapperchecker.KindForOfStatement:   r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	body := n.IterationBody()
	if body == nil {
		return
	}
	if alwaysExitsLoop(body) {
		ctx.Report(n, "Invalid loop. Its body allows only one iteration.")
	}
}

// alwaysExitsLoop reports whether every path through `n` ends in a
// statement that leaves the enclosing loop (`break`, `return`, or
// `throw`). `continue` is excluded — restarting the loop is not an
// exit.
func alwaysExitsLoop(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindBreakStatement,
		wrapperchecker.KindReturnStatement,
		wrapperchecker.KindThrowStatement:
		return true
	case wrapperchecker.KindBlock:
		return blockAlwaysExits(n)
	case wrapperchecker.KindIfStatement:
		thn := n.IfThen()
		els := n.IfElse()
		if thn == nil || els == nil {
			return false
		}
		return alwaysExitsLoop(thn) && alwaysExitsLoop(els)
	case wrapperchecker.KindLabeledStatement:
		return alwaysExitsLoop(labeledBody(n))
	case wrapperchecker.KindTryStatement:
		return tryAlwaysExits(n)
	}
	return false
}

func blockAlwaysExits(block *wrapperchecker.Node) bool {
	var stmts []*wrapperchecker.Node
	block.ForEachChild(func(c *wrapperchecker.Node) bool {
		stmts = append(stmts, c)
		return false
	})
	for _, s := range stmts {
		if alwaysExitsLoop(s) {
			return true
		}
		if isLoopContinuationExit(s) {
			// `continue` here means subsequent statements never
			// execute and the loop restarts. The body does NOT exit
			// the loop in this path.
			return false
		}
	}
	return false
}

func isLoopContinuationExit(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindContinueStatement
}

func tryAlwaysExits(try *wrapperchecker.Node) bool {
	var tryBlock, catchClause, finallyBlock *wrapperchecker.Node
	try.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindBlock:
			if tryBlock == nil {
				tryBlock = c
			} else if finallyBlock == nil {
				finallyBlock = c
			}
		case wrapperchecker.KindCatchClause:
			catchClause = c
		}
		return false
	})
	// If `finally` always exits, the try statement as a whole always
	// exits regardless of the try/catch arms.
	if finallyBlock != nil && alwaysExitsLoop(finallyBlock) {
		return true
	}
	// Otherwise require: try block exits AND (no catch OR catch
	// block exits).
	if tryBlock == nil || !alwaysExitsLoop(tryBlock) {
		return false
	}
	if catchClause == nil {
		return true
	}
	var catchBody *wrapperchecker.Node
	catchClause.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			catchBody = c
		}
		return false
	})
	return catchBody != nil && alwaysExitsLoop(catchBody)
}

func labeledBody(n *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindIdentifier {
			body = c
		}
		return false
	})
	return body
}
