// Package nounnecessaryqualifier implements the
// no-unnecessary-qualifier rule: flag namespace prefixes inside the
// same namespace.
package nounnecessaryqualifier

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-qualifier"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindQualifiedName:            visitQualified,
		wrapperchecker.KindPropertyAccessExpression: visitPropertyAccess,
	}
}

func visitQualified(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n, qualifiedLeft(n))
}

func visitPropertyAccess(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n, n.PropertyAccessReceiver())
}

func check(ctx *engine.Context, n, prefix *wrapperchecker.Node) {
	if prefix == nil {
		return
	}
	if prefix.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	prefixName := prefix.LiteralText()
	if prefixName == "" {
		return
	}
	if !enclosedByNamespace(n, prefixName) {
		return
	}
	ctx.Report(n, "qualifier is unnecessary inside the enclosing namespace")
}

// enclosedByNamespace reports whether n sits inside a namespace named
// `name`, walking up the AST.
func enclosedByNamespace(n *wrapperchecker.Node, name string) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		if cur.Kind() == wrapperchecker.KindModuleDeclaration {
			if moduleName(cur) == name {
				return true
			}
		}
	}
	return false
}

func qualifiedLeft(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		first = c
		return true
	})
	return first
}

func moduleName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
