// Package nounnecessarytypeparameters implements the
// no-unnecessary-type-parameters rule: flag a generic function whose
// type parameter appears only once in its signature. A single-use
// type parameter doesn't relate input to output and is equivalent to
// `unknown` (or its constraint).
//
// The upstream rule uses a full type-relation algorithm to handle
// indirect references (e.g. `T` reachable through a generic alias).
// This implementation runs the AST-based quick path: count textual
// references in the signature's parameters, return type, and other
// type-parameter constraints; flag when the count is one or zero.
package nounnecessarytypeparameters

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-type-parameters"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
		wrapperchecker.KindMethodSignature:     visit,
		wrapperchecker.KindCallSignature:       visit,
		wrapperchecker.KindConstructSignature:  visit,
		wrapperchecker.KindFunctionType:        visit,
		wrapperchecker.KindConstructorType:     visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	tparams := typeParametersOf(n)
	if len(tparams) == 0 {
		return
	}
	for _, tp := range tparams {
		name := typeParameterName(tp)
		if name == "" {
			continue
		}
		uses := countTypeParameterUsesInSignature(n, tp, name)
		if uses > 1 {
			continue
		}
		ctx.Report(tp, "type parameter `"+name+"` is used only once — replace usages with its constraint or `unknown`")
	}
}

// typeParametersOf returns the explicit type-parameter declarations
// of a function-like signature, or nil for none.
func typeParametersOf(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindTypeParameter {
			out = append(out, c)
		}
		return false
	})
	return out
}

// typeParameterName returns the identifier text of a TypeParameter
// declaration (the `T` in `<T extends U = D>`). Empty when malformed.
func typeParameterName(tp *wrapperchecker.Node) string {
	var name string
	tp.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// countTypeParameterUsesInSignature counts identifier references with
// the given name within fn's parameter type-annotations, return-type
// annotation, and the constraints/defaults of OTHER type parameters
// (a constraint like `<T, U extends T>` makes T count twice). The
// type-parameter's own name slot is excluded so the declaration
// itself doesn't count.
func countTypeParameterUsesInSignature(fn, owner *wrapperchecker.Node, name string) int {
	count := 0
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier && n.LiteralText() == name {
			count++
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindParameter:
			if annot := c.ParameterTypeAnnotation(); annot != nil {
				walk(annot)
			}
		case wrapperchecker.KindTypeParameter:
			if c == owner {
				return false
			}
			// Constraint / default of a sibling type parameter — walk
			// every child except the parameter's own name identifier.
			first := true
			c.ForEachChild(func(part *wrapperchecker.Node) bool {
				if first && part.Kind() == wrapperchecker.KindIdentifier {
					first = false
					return false
				}
				first = false
				walk(part)
				return false
			})
		}
		return false
	})
	if rt := returnTypeAnnotation(fn); rt != nil {
		walk(rt)
	}
	return count
}

// returnTypeAnnotation returns the explicit return-type-annotation node
// of a function-like signature, or nil when inferred or absent. The
// return type is the type-shaped child that appears after the
// parameter list and before any function body.
func returnTypeAnnotation(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	sawParams := false
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindTypeParameter,
			wrapperchecker.KindParameter:
			sawParams = true
			return false
		case wrapperchecker.KindBlock:
			return true
		}
		if !sawParams {
			return false
		}
		if isTypeNodeKind(c.Kind()) {
			found = c
			return true
		}
		return false
	})
	return found
}

// isTypeNodeKind reports whether kind names a type-form node. Used to
// identify a return-type annotation among a function's children. The
// list isn't exhaustive — type-references show up as plain identifier
// nodes too — but for the function-child positions we care about, any
// child past the parameter list that isn't a body/name slot is a type
// annotation.
func isTypeNodeKind(kind wrapperchecker.Kind) bool {
	switch kind {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindTypeParameter,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBlock:
		return false
	}
	return true
}
