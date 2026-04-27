// Package noredundanttypeconstituents implements the
// no-redundant-type-constituents rule: flag `T | any`, `T | unknown`,
// `T & never`, etc. where one constituent makes the others irrelevant.
package noredundanttypeconstituents

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-redundant-type-constituents"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindUnionType:        visitUnion,
		wrapperchecker.KindIntersectionType: visitIntersection,
	}
}

func visitUnion(ctx *engine.Context, n *wrapperchecker.Node) {
	// In a union: `any` and `unknown` make every other member redundant.
	// Find the dominator and report each non-dominator member.
	var dominatorIdx = -1
	var dominatorName string
	var members []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		members = append(members, c)
		return false
	})
	for i, m := range members {
		t := ctx.Checker().TypeFromTypeNode(m)
		if t == nil {
			continue
		}
		if t.IsAny() {
			dominatorIdx = i
			dominatorName = "any"
			break
		}
		if t.IsUnknown() {
			dominatorIdx = i
			dominatorName = "unknown"
			break
		}
	}
	if dominatorIdx < 0 {
		return
	}
	for i, m := range members {
		if i == dominatorIdx {
			continue
		}
		ctx.Report(m, "type constituent is overridden by `"+dominatorName+"` in this union")
	}
}

func visitIntersection(ctx *engine.Context, n *wrapperchecker.Node) {
	// In an intersection: `never` makes every other member redundant.
	var dominatorIdx = -1
	var members []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		members = append(members, c)
		return false
	})
	for i, m := range members {
		t := ctx.Checker().TypeFromTypeNode(m)
		if t == nil {
			continue
		}
		if t.IsNever() {
			dominatorIdx = i
			break
		}
	}
	if dominatorIdx < 0 {
		return
	}
	for i, m := range members {
		if i == dominatorIdx {
			continue
		}
		ctx.Report(m, "type constituent is overridden by `never` in this intersection")
	}
}
