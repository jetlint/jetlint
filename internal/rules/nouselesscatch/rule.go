// Package nouselesscatch implements no-useless-catch: a `catch (e) {
// throw e; }` block adds no value — drop the try/catch (keep a
// finally if present).
package nouselesscatch

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-catch"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTryStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// TryStatement: tryBlock, catchClause?, finallyBlock?
	var catch *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindCatchClause {
			catch = c
		}
		return false
	})
	if catch == nil {
		return
	}
	// CatchClause children: VariableDeclaration (binding) and Block.
	var binding, body *wrapperchecker.Node
	catch.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindVariableDeclaration {
			binding = c
		} else if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	if binding == nil || body == nil {
		return
	}
	bindingName := bindingIdentName(binding)
	if bindingName == "" {
		return
	}
	// Body must be a single `throw <bindingName>;` statement.
	var stmts []*wrapperchecker.Node
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		stmts = append(stmts, c)
		return false
	})
	if len(stmts) != 1 {
		return
	}
	if stmts[0].Kind() != wrapperchecker.KindThrowStatement {
		return
	}
	var thrown *wrapperchecker.Node
	stmts[0].ForEachChild(func(c *wrapperchecker.Node) bool {
		if thrown == nil {
			thrown = c
		}
		return false
	})
	if thrown == nil || thrown.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if thrown.SourceText() != bindingName {
		return
	}
	ctx.Report(catch, "useless catch — block only rethrows the caught error")
}

func bindingIdentName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil || first.Kind() != wrapperchecker.KindIdentifier {
		return ""
	}
	return first.SourceText()
}
