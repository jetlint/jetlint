// Package nouselesscatchbinding implements no-useless-catch-binding:
// `catch (e)` where `e` is never used should just be `catch`.
package nouselesscatchbinding

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-catch-binding"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCatchClause: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var binding, body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
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
	name := bindingIdentName(binding)
	if name == "" {
		return
	}
	if usesIdentifier(body, name) {
		return
	}
	ctx.Report(binding, "unused catch binding — use bare `catch` instead")
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

func usesIdentifier(n *wrapperchecker.Node, name string) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindIdentifier && n.SourceText() == name {
		return true
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if usesIdentifier(c, name) {
			found = true
			return true
		}
		return false
	})
	return found
}
