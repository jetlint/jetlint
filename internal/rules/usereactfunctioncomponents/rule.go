// Package usereactfunctioncomponents implements
// use-react-function-components: class components are a legacy
// pattern except for error boundaries (which need `componentDidCatch`).
package usereactfunctioncomponents

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-react-function-components"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: visit,
		wrapperchecker.KindClassExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !extendsReactComponent(n) {
		return
	}
	if hasComponentDidCatch(n) {
		return
	}
	ctx.Report(n, "rewrite this class component as a function component")
}

func extendsReactComponent(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindHeritageClause {
			src := c.SourceText()
			if !strings.HasPrefix(strings.TrimSpace(src), "extends") {
				return false
			}
			if strings.Contains(src, "React.Component") ||
				strings.Contains(src, "React.PureComponent") ||
				strings.Contains(src, "Component") {
				found = true
			}
		}
		return false
	})
	return found
}

func hasComponentDidCatch(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindMethodDeclaration {
			return false
		}
		var name *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if name == nil {
				name = d
			}
			return false
		})
		if name != nil && name.Kind() == wrapperchecker.KindIdentifier && name.SourceText() == "componentDidCatch" {
			found = true
		}
		return false
	})
	return found
}
