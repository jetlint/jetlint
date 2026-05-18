// Package nolabelvar implements no-label-var: a statement label that
// shares its name with a variable in scope is confusing to read.
package nolabelvar

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-label-var"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindLabeledStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var label *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if label == nil && c.Kind() == wrapperchecker.KindIdentifier {
			label = c
		}
		return false
	})
	if label == nil {
		return
	}
	name := label.SourceText()
	for p := n.Parent(); p != nil; p = p.Parent() {
		if declaresVar(p, name) {
			ctx.Report(label, "label `"+name+"` shadows a variable in scope")
			return
		}
	}
}

func declaresVar(scope *wrapperchecker.Node, name string) bool {
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindVariableStatement {
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() == wrapperchecker.KindVariableDeclarationList {
					d.ForEachChild(func(decl *wrapperchecker.Node) bool {
						var first *wrapperchecker.Node
						decl.ForEachChild(func(x *wrapperchecker.Node) bool {
							if first == nil {
								first = x
							}
							return false
						})
						if first != nil && first.Kind() == wrapperchecker.KindIdentifier && first.SourceText() == name {
							found = true
						}
						return false
					})
				}
				return false
			})
		}
		return false
	})
	return found
}
