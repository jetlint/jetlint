// Package nounsafedeclarationmerging implements
// no-unsafe-declaration-merging: an interface and a class sharing one
// name merge in unexpected ways at runtime — give them distinct names.
package nounsafedeclarationmerging

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unsafe-declaration-merging"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
		wrapperchecker.KindBlock:      visit,
		wrapperchecker.KindModuleBlock: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	interfaces := map[string]*wrapperchecker.Node{}
	classes := map[string]*wrapperchecker.Node{}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindInterfaceDeclaration:
			if name := declName(c); name != "" {
				interfaces[name] = c
			}
		case wrapperchecker.KindClassDeclaration:
			if c.HasDeclareModifier() || ancestorIsDeclareModule(c) {
				return false
			}
			if name := declName(c); name != "" {
				classes[name] = c
			}
		}
		return false
	})
	for name, cls := range classes {
		if _, ok := interfaces[name]; ok {
			ctx.Report(cls, "class `"+name+"` shares a name with an interface — they merge unsafely")
		}
	}
}

func ancestorIsDeclareModule(start *wrapperchecker.Node) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindModuleDeclaration && p.HasDeclareModifier() {
			return true
		}
	}
	return false
}

func declName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil && c.Kind() == wrapperchecker.KindIdentifier {
			first = c
		}
		return false
	})
	if first == nil {
		return ""
	}
	return first.SourceText()
}
