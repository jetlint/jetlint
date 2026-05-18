// Package nouselesscontinue implements no-useless-continue: a
// `continue` whose only effect is to skip to the next loop iteration
// when execution would already reach there is redundant.
package nouselesscontinue

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-continue"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindContinueStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Walk up: check that `n` is the last statement in its parent block
	// (or a single statement parent); then ascend through if-branches
	// until we reach a loop body.
	cur := n
	for {
		parent := cur.Parent()
		if parent == nil {
			return
		}
		switch parent.Kind() {
		case wrapperchecker.KindBlock:
			if !isLastChild(parent, cur) {
				return
			}
			cur = parent
			continue
		case wrapperchecker.KindIfStatement:
			// `cur` must be one of the branches.
			var test, then, els *wrapperchecker.Node
			i := 0
			parent.ForEachChild(func(c *wrapperchecker.Node) bool {
				switch i {
				case 0:
					test = c
				case 1:
					then = c
				case 2:
					els = c
				}
				i++
				return false
			})
			_ = test
			if cur != then && cur != els {
				return
			}
			cur = parent
			continue
		case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement,
			wrapperchecker.KindForStatement, wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement:
			// Reached a loop body. If our continue has a label, it must match.
			if !labelOK(n, parent) {
				return
			}
			ctx.Report(n, "useless `continue` — control falls through to the next iteration anyway")
			return
		case wrapperchecker.KindLabeledStatement:
			cur = parent
			continue
		default:
			return
		}
	}
}

func labelOK(continueStmt, loop *wrapperchecker.Node) bool {
	var label *wrapperchecker.Node
	continueStmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			label = c
		}
		return false
	})
	if label == nil {
		return true
	}
	// Label must match the loop's enclosing labeled statement.
	lp := loop.Parent()
	if lp == nil || lp.Kind() != wrapperchecker.KindLabeledStatement {
		return false
	}
	var first *wrapperchecker.Node
	lp.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil || first.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	return first.SourceText() == label.SourceText()
}

func isLastChild(parent, child *wrapperchecker.Node) bool {
	var last *wrapperchecker.Node
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last == child
}
