// Package noduplicatetypeconstituents implements the
// no-duplicate-type-constituents rule: flag duplicate types in a
// union or intersection.
package noduplicatetypeconstituents

import (
	"strings"

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
	// Skip nested same-kind unions/intersections that are inside
	// parens of an outer union/intersection — the outer visit will
	// inspect all groups, treating each parenthesized sub-union as one
	// member of the outer.
	if isNestedSameKind(n) {
		return
	}
	// Optional parameter: `(a?: string | undefined)` — the `?` already
	// adds `undefined`, so a literal `undefined` member anywhere in the
	// (possibly nested) union is duplicate.
	if n.Kind() == wrapperchecker.KindUnionType && unionIsOptionalParameterType(n) {
		var allLeaves []*wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			collectLeaves(c, n.Kind(), &allLeaves)
			return false
		})
		for _, m := range allLeaves {
			t := ctx.Checker().TypeFromTypeNode(m)
			if t != nil && t.IsNullOrUndefined() && t.String() == "undefined" {
				ctx.Report(m, "duplicate type constituent — `undefined` is already implied by the optional `?` parameter marker")
				break
			}
		}
	}
	if n.Kind() == wrapperchecker.KindUnionType {
		visitUnionGrouped(ctx, n)
		return
	}
	visitIntersectionFlattened(ctx, n)
}

// visitUnionGrouped: a union member is a duplicate when its full
// leaf-set is already represented by earlier members (group-level
// dedup). This matches upstream's reporting style for unions, where
// `(A | B) | (A | B)` is one error pointing at the duplicate group.
func visitUnionGrouped(ctx *engine.Context, n *wrapperchecker.Node) {
	var directs []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		directs = append(directs, c)
		return false
	})
	seen := map[string]bool{}
	for _, m := range directs {
		leafKeys, ok := memberLeafKeys(ctx, m, n.Kind())
		if !ok || len(leafKeys) == 0 {
			continue
		}
		allSeen := true
		for _, k := range leafKeys {
			if !seen[k] {
				allSeen = false
				break
			}
		}
		if allSeen {
			previous := leafKeys[0]
			if len(leafKeys) > 1 {
				previous = "{" + strings.Join(leafKeys, " | ") + "}"
			}
			ctx.Report(m, "duplicate type constituent — `"+previous+"` already appears earlier in this union")
			continue
		}
		for _, k := range leafKeys {
			seen[k] = true
		}
	}
}

// visitIntersectionFlattened: intersections dedup at the leaf level —
// `number & string & (number & string)` reports the inner `number`
// and `string` individually, not the parenthesized group as a single
// duplicate.
func visitIntersectionFlattened(ctx *engine.Context, n *wrapperchecker.Node) {
	var leaves []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		collectLeaves(c, n.Kind(), &leaves)
		return false
	})
	seen := map[string]bool{}
	for _, m := range leaves {
		t := ctx.Checker().TypeFromTypeNode(m)
		if t == nil {
			continue
		}
		if t.IsAny() && m.Kind() != wrapperchecker.KindAnyKeyword {
			continue
		}
		key := t.String()
		if key == "" {
			continue
		}
		if seen[key] {
			ctx.Report(m, "duplicate type constituent — `"+key+"` already appears earlier in this intersection")
			continue
		}
		seen[key] = true
	}
}

// memberLeafKeys returns the canonical type keys of the leaves of one
// direct union/intersection member, descending through parens and
// nested same-kind groups. Returns ok=false when any leaf is an
// unresolved-`any` whose duplicate-ness is purely syntactic — TS's
// own diagnostics will already complain.
func memberLeafKeys(ctx *engine.Context, member *wrapperchecker.Node, parentKind wrapperchecker.Kind) ([]string, bool) {
	var leaves []*wrapperchecker.Node
	collectLeaves(member, parentKind, &leaves)
	out := make([]string, 0, len(leaves))
	for _, lf := range leaves {
		t := ctx.Checker().TypeFromTypeNode(lf)
		if t == nil {
			return nil, false
		}
		if t.IsAny() && lf.Kind() != wrapperchecker.KindAnyKeyword {
			return nil, false
		}
		k := t.String()
		if k == "" {
			return nil, false
		}
		out = append(out, k)
	}
	return out, true
}

// collectLeaves flattens one direct union/intersection member through
// parens and nested same-kind groups so each terminal type appears
// once in the leaf list.
func collectLeaves(member *wrapperchecker.Node, parentKind wrapperchecker.Kind, out *[]*wrapperchecker.Node) {
	switch member.Kind() {
	case wrapperchecker.KindParenthesizedType:
		member.ForEachChild(func(c *wrapperchecker.Node) bool {
			collectLeaves(c, parentKind, out)
			return false
		})
		return
	case parentKind:
		member.ForEachChild(func(c *wrapperchecker.Node) bool {
			collectLeaves(c, parentKind, out)
			return false
		})
		return
	}
	*out = append(*out, member)
}

// isNestedSameKind reports whether n is a union/intersection whose
// closest non-parenthesized ancestor is the same kind. The outer
// container's visit will already see all leaf members through
// flattening.
func isNestedSameKind(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		if cur.Kind() == wrapperchecker.KindParenthesizedType {
			continue
		}
		return cur.Kind() == n.Kind()
	}
	return false
}

// unionIsOptionalParameterType walks up through parens/aliases to see
// whether the union is the type annotation of an optional parameter.
func unionIsOptionalParameterType(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindParenthesizedType:
			continue
		case wrapperchecker.KindParameter:
			return cur.IsOptionalParameter()
		default:
			return false
		}
	}
	return false
}

