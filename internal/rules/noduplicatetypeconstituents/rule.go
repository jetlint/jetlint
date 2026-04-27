// Package noduplicatetypeconstituents implements the
// no-duplicate-type-constituents rule: flag duplicate types in a
// union or intersection.
package noduplicatetypeconstituents

import (
	"strconv"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-duplicate-type-constituents"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindUnionType:        visit,
		wrapperchecker.KindIntersectionType: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var members []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		members = append(members, c)
		return false
	})
	seen := map[string]int{}
	for i, m := range members {
		t := ctx.Checker().TypeFromTypeNode(m)
		if t == nil {
			continue
		}
		key := t.String()
		if key == "" {
			continue
		}
		if first, ok := seen[key]; ok {
			ctx.Report(m, "duplicate type constituent — `"+key+"` already appears at position "+strconv.Itoa(first+1))
			continue
		}
		seen[key] = i
	}
}
