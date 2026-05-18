// Package guardforin implements the guard-for-in rule: a for-in loop
// should filter inherited properties from the prototype chain, either
// by guarding its body with an `if` or by skipping iterations with
// `continue`. Iterating an object's enumerable keys without filtering
// pulls in everything from the prototype, which is almost never what
// the author intends.
//
// Matches oxlint and ESLint's guard-for-in. Zero options, syntactic
// check only. Accepted body shapes (mirrored from upstream):
//
//   - EmptyStatement: `for (var x in o);`
//   - IfStatement directly: `for (var x in o) if (x) f();`
//   - Empty Block: `for (var x in o) {}`
//   - Block containing exactly one IfStatement
//   - Block whose first statement is `if (...) continue;` (raw or
//     `{ continue; }`); everything after the if is therefore guarded
//
// Anything else is reported.
package guardforin

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "guard-for-in"

// New constructs a guard-for-in rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindForInStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	body := forInBody(n)
	if body == nil {
		return
	}
	if isAcceptableBody(body) {
		return
	}
	ctx.Report(n, "for-in loops should filter properties using an if statement")
}

// forInBody returns the statement body of a ForInStatement. Children
// appear in source order: initializer/declaration, iterated expression,
// body. The body is therefore the last child.
func forInBody(loop *wrapperchecker.Node) *wrapperchecker.Node {
	var last *wrapperchecker.Node
	loop.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

func isAcceptableBody(body *wrapperchecker.Node) bool {
	switch body.Kind() {
	case wrapperchecker.KindIfStatement:
		return true
	case wrapperchecker.KindBlock:
		return isAcceptableBlockBody(body)
	default:
		// EmptyStatement (`;`) has no children; ExpressionStatement
		// and other statements have at least one. Treating "no
		// children" as the empty-body case avoids needing a dedicated
		// Kind constant for EmptyStatement.
		return !hasChildren(body)
	}
}

func isAcceptableBlockBody(block *wrapperchecker.Node) bool {
	stmts := block.BlockStatements()
	switch len(stmts) {
	case 0:
		return true
	case 1:
		return stmts[0].Kind() == wrapperchecker.KindIfStatement
	}
	first := stmts[0]
	if first.Kind() != wrapperchecker.KindIfStatement {
		return false
	}
	// `if (x) continue;` and `if (x) { continue; }` both guard the
	// rest of the block: any non-matching iteration skips the
	// trailing statements.
	return isContinueGuard(first.IfThen())
}

func isContinueGuard(consequent *wrapperchecker.Node) bool {
	if consequent == nil {
		return false
	}
	if consequent.Kind() == wrapperchecker.KindContinueStatement {
		return true
	}
	if consequent.Kind() != wrapperchecker.KindBlock {
		return false
	}
	stmts := consequent.BlockStatements()
	return len(stmts) == 1 && stmts[0].Kind() == wrapperchecker.KindContinueStatement
}

func hasChildren(n *wrapperchecker.Node) bool {
	seen := false
	n.ForEachChild(func(*wrapperchecker.Node) bool {
		seen = true
		return true
	})
	return seen
}
