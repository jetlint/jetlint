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
	// Skip nested same-kind unions/intersections that are inside
	// parens of an outer union/intersection — the outer visit will
	// inspect all leaf members. Without this, `A | (A | A)` reports
	// twice (once on outer, once on inner).
	if isNestedSameKind(n) {
		return
	}
	var members []*wrapperchecker.Node
	collect(n, n.Kind(), &members)
	// Optional parameter: `(a?: string | undefined)` — the `?` already
	// adds `undefined`, so a literal `undefined` member is duplicate.
	if n.Kind() == wrapperchecker.KindUnionType && unionIsOptionalParameterType(n) {
		for _, m := range members {
			t := ctx.Checker().TypeFromTypeNode(m)
			if t != nil && t.IsNullOrUndefined() && t.String() == "undefined" {
				ctx.Report(m, "duplicate type constituent — `undefined` is already implied by the optional `?` parameter marker")
				break
			}
		}
	}
	seen := map[string]int{}
	for i, m := range members {
		t := ctx.Checker().TypeFromTypeNode(m)
		if t == nil {
			continue
		}
		// Don't flag duplicates whose type couldn't be resolved (e.g.
		// `A | A` where A is undeclared). Both sides become `any` from
		// the checker's perspective, but the duplication is purely
		// syntactic and the rule defers to TypeScript's own diagnostics.
		// Explicit `any` keywords are still flagged — they're a real
		// duplicate the user wrote out twice.
		if t.IsAny() && m.Kind() != wrapperchecker.KindAnyKeyword {
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

// collect walks a union/intersection, descending through parens and
// nested same-kind unions/intersections so each leaf is appended once.
func collect(n *wrapperchecker.Node, kind wrapperchecker.Kind, out *[]*wrapperchecker.Node) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindParenthesizedType:
			collect(c, kind, out)
		case kind:
			collect(c, kind, out)
		default:
			*out = append(*out, c)
		}
		return false
	})
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

