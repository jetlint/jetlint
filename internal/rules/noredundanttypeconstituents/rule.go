// Package noredundanttypeconstituents implements the
// no-redundant-type-constituents rule: flag `T | any`, `T | unknown`,
// `T & never`, etc. where one constituent makes the others irrelevant.
package noredundanttypeconstituents

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
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
	// Direct children only — nested unions get visited independently
	// when the walker reaches them.
	var members []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		members = append(members, c)
		return false
	})
	types := make([]*wrapperchecker.Type, len(members))
	for i, m := range members {
		types[i] = ctx.Checker().TypeFromTypeNode(m)
	}
	// First: dominators (any / unknown) collapse the entire union.
	for i, t := range types {
		if t == nil {
			continue
		}
		if t.IsAny() || t.IsUnknown() {
			name := "unknown"
			if t.IsAny() {
				name = "any"
			}
			for j, m := range members {
				if j == i {
					continue
				}
				ctx.Report(m, "type constituent is overridden by `"+name+"` in this union")
			}
			return
		}
	}
	// `never` is the union identity — every never is redundant in a
	// regular position. Function return-type unions are exempt:
	// `() => string | never` documents the function may throw.
	if !isFunctionReturnTypeUnion(n) {
		for i, t := range types {
			if t == nil {
				continue
			}
			if t.IsNever() {
				ctx.Report(members[i], "`never` is redundant as a union constituent")
			}
		}
	}
	// Literal types subsumed by a primitive sibling: `number | 0` →
	// 0 is redundant.
	primitives := primitiveKindsPresent(types)
	for i, t := range types {
		if t == nil {
			continue
		}
		if t.IsNever() {
			continue
		}
		if k := literalPrimitiveKind(t); k != "" && primitives[k] {
			ctx.Report(members[i], "literal type is redundant — its primitive parent already appears in the union")
			continue
		}
		// Mixed-kind sub-union member: if any of its leaves is a
		// literal of a primitive kind already present in the outer
		// union, the whole member contains redundant constituents.
		// `(2 | 'other' | 3) | number` — flag the parenthesized group.
		if t.IsUnion() && unionContainsLiteralOfKind(t, primitives) {
			ctx.Report(members[i], "literal type is redundant — its primitive parent already appears in the union")
		}
	}
}

// unionContainsLiteralOfKind reports whether any leaf member of a
// union is a literal whose primitive kind is present in the supplied
// set. Walks through nested unions.
func unionContainsLiteralOfKind(t *wrapperchecker.Type, primitives map[string]bool) bool {
	if t == nil || !t.IsUnion() {
		return false
	}
	// `boolean` is modeled internally as `true | false`. A naked
	// primitive should not be treated as containing redundant literal
	// leaves of itself.
	switch t.String() {
	case "string", "number", "bigint", "boolean":
		return false
	}
	for _, m := range t.UnionMembers() {
		if m.IsUnion() {
			if unionContainsLiteralOfKind(m, primitives) {
				return true
			}
			continue
		}
		if k := literalPrimitiveKind(m); k != "" && primitives[k] {
			return true
		}
	}
	return false
}

// collectUnionMembers walks the syntactic tree of a union, flattening
// nested unions and parenthesized type wrappers so each leaf member
// can be checked individually.
func collectUnionMembers(n *wrapperchecker.Node, out *[]*wrapperchecker.Node) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindUnionType:
			collectUnionMembers(c, out)
		case wrapperchecker.KindParenthesizedType:
			c.ForEachChild(func(inner *wrapperchecker.Node) bool {
				if inner.Kind() == wrapperchecker.KindUnionType {
					collectUnionMembers(inner, out)
				} else {
					*out = append(*out, inner)
				}
				return false
			})
		default:
			*out = append(*out, c)
		}
		return false
	})
}

// isFunctionReturnTypeUnion reports whether the union node sits in
// the return-type position of a function-like declaration, walking
// through parenthesized type wrappers.
func isFunctionReturnTypeUnion(n *wrapperchecker.Node) bool {
	parent := n.Parent()
	for parent != nil {
		switch parent.Kind() {
		case wrapperchecker.KindFunctionType,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindCallSignature,
			wrapperchecker.KindMethodSignature,
			wrapperchecker.KindConstructSignature,
			wrapperchecker.KindConstructorType:
			return true
		case wrapperchecker.KindParenthesizedType,
			wrapperchecker.KindTypeLiteral:
			parent = parent.Parent()
			continue
		}
		return false
	}
	return false
}

func primitiveKindsPresent(ts []*wrapperchecker.Type) map[string]bool {
	out := map[string]bool{}
	for _, t := range ts {
		if t == nil {
			continue
		}
		// Only treat exact primitive types as the dominator — literal
		// types must NOT match themselves.
		if t.String() == "string" {
			out["string"] = true
		} else if t.String() == "number" {
			out["number"] = true
		} else if t.String() == "bigint" {
			out["bigint"] = true
		} else if t.String() == "boolean" {
			out["boolean"] = true
		}
	}
	return out
}

func literalPrimitiveKind(t *wrapperchecker.Type) string {
	switch t.String() {
	case "string", "number", "bigint", "boolean", "null", "undefined", "object", "symbol":
		// The primitive type itself — never a redundant literal.
		return ""
	}
	if t.IsStringLike() {
		return "string"
	}
	if t.IsNumberLike() {
		return "number"
	}
	if t.IsBigIntLike() {
		return "bigint"
	}
	if t.IsBooleanLike() {
		return "boolean"
	}
	if t.IsUnion() {
		// Sub-union: every member must classify as the same primitive
		// for the whole sub-union to be subsumed by a sibling.
		var seen string
		for _, m := range t.UnionMembers() {
			k := literalPrimitiveKind(m)
			if k == "" {
				return ""
			}
			if seen == "" {
				seen = k
				continue
			}
			if seen != k {
				return ""
			}
		}
		return seen
	}
	return ""
}

func visitIntersection(ctx *engine.Context, n *wrapperchecker.Node) {
	var members []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		members = append(members, c)
		return false
	})
	types := make([]*wrapperchecker.Type, len(members))
	for i, m := range members {
		types[i] = ctx.Checker().TypeFromTypeNode(m)
	}
	// `never` collapses the entire intersection — every sibling is
	// redundant.
	for i, t := range types {
		if t == nil {
			continue
		}
		if t.IsNever() {
			for j, m := range members {
				if j == i {
					continue
				}
				ctx.Report(m, "type constituent is overridden by `never` in this intersection")
			}
			return
		}
	}
	// `any`/`unknown`/`{}` are subsumed by any other constituent in an
	// intersection — they don't constrain anything.
	for i, t := range types {
		if t == nil {
			continue
		}
		if t.IsUnknown() {
			ctx.Report(members[i], "`unknown` is redundant as an intersection constituent")
			continue
		}
		if t.IsAny() {
			ctx.Report(members[i], "`any` overrides the other constituents of this intersection")
			continue
		}
	}
	// Primitive subsumed by a narrower literal sibling: in `boolean &
	// false`, boolean is redundant. Detect when a member is one of the
	// known primitive types and another member is a literal of that
	// kind.
	literalKinds := map[string]bool{}
	for _, t := range types {
		if t == nil {
			continue
		}
		if k := literalPrimitiveKind(t); k != "" {
			literalKinds[k] = true
		}
	}
	for i, t := range types {
		if t == nil {
			continue
		}
		k := primitiveKindOfType(t)
		if k == "" {
			continue
		}
		if literalKinds[k] {
			ctx.Report(members[i], "primitive constituent is redundant — a narrower literal of the same kind is also in the intersection")
		}
	}
}

// primitiveKindOfType returns "string"/"number"/"bigint"/"boolean"
// when t is exactly that primitive type, or "" otherwise.
func primitiveKindOfType(t *wrapperchecker.Type) string {
	switch t.String() {
	case "string":
		return "string"
	case "number":
		return "number"
	case "bigint":
		return "bigint"
	case "boolean":
		return "boolean"
	}
	return ""
}
