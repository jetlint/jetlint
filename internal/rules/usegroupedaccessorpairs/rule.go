// Package usegroupedaccessorpairs implements use-grouped-accessor-pairs:
// `get x` and `set x` belong next to each other so the pair is
// discoverable at a glance.
package usegroupedaccessorpairs

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-grouped-accessor-pairs"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration:        visit,
		wrapperchecker.KindClassExpression:         visit,
		wrapperchecker.KindObjectLiteralExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Walk members in order; record last index seen for each (name, kind).
	type marker struct {
		name string
		idx  int
		node *wrapperchecker.Node
	}
	var gets, sets []marker
	i := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindGetAccessor:
			gets = append(gets, marker{name: accessorName(c), idx: i, node: c})
			i++
		case wrapperchecker.KindSetAccessor:
			sets = append(sets, marker{name: accessorName(c), idx: i, node: c})
			i++
		default:
			i++
		}
		return false
	})
	for _, g := range gets {
		for _, s := range sets {
			if g.name == s.name && g.name != "" {
				diff := g.idx - s.idx
				if diff != 1 && diff != -1 {
					ctx.Report(g.node, "place get/set for `"+g.name+"` next to each other")
					break
				}
			}
		}
	}
}

func accessorName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
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
	if first.Kind() == wrapperchecker.KindStringLiteral || first.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return first.LiteralText()
	}
	return ""
}
