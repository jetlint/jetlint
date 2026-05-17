// Package nocatchassign implements no-catch-assign: reassigning the
// caught exception loses the original — and on legacy engines the
// rebinding can leak out of the catch block.
package nocatchassign

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-catch-assign"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := operatorToken(n)
	if op != "=" && op != "+=" && op != "-=" && op != "*=" && op != "/=" && op != "&&=" && op != "||=" && op != "??=" {
		return
	}
	target := leftOperand(n)
	if target == nil || target.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	name := target.SourceText()
	// Walk up to find enclosing CatchClause.
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindCatchClause {
			// CatchClause children: [VariableDeclaration (binding), Block].
			caught := catchBindingName(p)
			if caught == name {
				ctx.Report(n, "reassigning caught exception `"+name+"` loses the original error")
			}
			return
		}
		// Stop at function boundaries.
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration:
			return
		}
	}
}

func leftOperand(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return first
}

func operatorToken(n *wrapperchecker.Node) string {
	var second *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 1 {
			second = c
			return true
		}
		idx++
		return false
	})
	if second == nil {
		return ""
	}
	return second.SourceText()
}

func catchBindingName(catch *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	catch.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil {
		return ""
	}
	if first.Kind() == wrapperchecker.KindIdentifier {
		return first.SourceText()
	}
	// VariableDeclaration wrapping the name.
	if first.Kind() == wrapperchecker.KindVariableDeclaration {
		var inner *wrapperchecker.Node
		first.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner != nil && inner.Kind() == wrapperchecker.KindIdentifier {
			return inner.SourceText()
		}
	}
	return ""
}
