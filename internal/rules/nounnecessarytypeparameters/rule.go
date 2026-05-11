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
	// First pass: collect parameter names whose annotations reference
	// the type parameter — these are the parameters that "carry" T
	// through `typeof`/`keyof typeof` indirection in the return type.
	paramsByT := map[string]bool{}
	count := 0
	var walk func(n *wrapperchecker.Node, inTypeQuery bool)
	walk = func(n *wrapperchecker.Node, inTypeQuery bool) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier {
			text := n.LiteralText()
			if text == name {
				count++
				return
			}
			if inTypeQuery && paramsByT[text] {
				// `typeof param` where param: ...T... — semantic use.
				count++
				return
			}
		}
		isTQ := inTypeQuery || n.Kind() == wrapperchecker.KindTypeQuery
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c, isTQ)
			return false
		})
	}
	// First: gather param name → carries T.
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindParameter {
			return false
		}
		annot := c.ParameterTypeAnnotation()
		if annot == nil {
			return false
		}
		before := count
		walk(annot, false)
		if count > before {
			if pname := parameterIdentifier(c); pname != "" {
				paramsByT[pname] = true
			}
		}
		return false
	})
	// Now visit sibling type parameter constraints/defaults.
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindTypeParameter || c == owner {
			return false
		}
		first := true
		c.ForEachChild(func(part *wrapperchecker.Node) bool {
			if first && part.Kind() == wrapperchecker.KindIdentifier {
				first = false
				return false
			}
			first = false
			walk(part, false)
			return false
		})
		return false
	})
	if rt := returnTypeAnnotation(fn); rt != nil {
		walk(rt, false)
	} else if count > 0 && len(paramsByT) > 0 {
		// No explicit return type — check whether the body returns one
		// of the T-carrying parameters in a SHAPE-preserving way:
		// bare `return param` keeps T in the inferred return type,
		// `return { x: param }` produces `{ x: T }`, `return [param]`
		// produces `T[]`. Calls on the parameter (`param.toString()`)
		// or arithmetic don't escape T — they produce a concrete type.
		if body := fn.FunctionBody(); body != nil {
			var bodyWalk func(n *wrapperchecker.Node)
			added := false
			bodyWalk = func(n *wrapperchecker.Node) {
				if n == nil || added {
					return
				}
				if n.Kind() == wrapperchecker.KindReturnStatement {
					n.ForEachChild(func(c *wrapperchecker.Node) bool {
						if returnEscapesT(c, paramsByT) {
							count++
							added = true
						}
						return added
					})
				}
				if n.Kind() == wrapperchecker.KindFunctionDeclaration ||
					n.Kind() == wrapperchecker.KindFunctionExpression ||
					n.Kind() == wrapperchecker.KindArrowFunction {
					// Don't descend into nested functions — their
					// returns belong to their own signatures.
					return
				}
				n.ForEachChild(func(c *wrapperchecker.Node) bool {
					bodyWalk(c)
					return added
				})
			}
			bodyWalk(body)
		}
	}
	return count
}

// returnEscapesT reports whether the given return-expression
// shape-preserves a T-carrying parameter into its inferred type.
// Only structural carriers count: a bare identifier, an object
// literal whose property values escape T, an array literal whose
// elements escape T, an `as`/parenthesized wrapper, or a spread of
// an escaping expression. Calls, member access, arithmetic, and
// other transforming operators produce a fresh type that no longer
// carries T even though the parameter is mentioned in the source.
func returnEscapesT(e *wrapperchecker.Node, paramsByT map[string]bool) bool {
	if e == nil {
		return false
	}
	switch e.Kind() {
	case wrapperchecker.KindIdentifier:
		return paramsByT[e.LiteralText()]
	case wrapperchecker.KindParenthesizedExpression,
		wrapperchecker.KindAsExpression,
		wrapperchecker.KindTypeAssertionExpression,
		wrapperchecker.KindNonNullExpression,
		wrapperchecker.KindSatisfiesExpression:
		var inner *wrapperchecker.Node
		e.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
				return true
			}
			return false
		})
		return returnEscapesT(inner, paramsByT)
	case wrapperchecker.KindObjectLiteralExpression:
		found := false
		e.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindPropertyAssignment:
				// `{ key: value }` — only the value carries the type.
				var val *wrapperchecker.Node
				seenName := false
				c.ForEachChild(func(p *wrapperchecker.Node) bool {
					if !seenName {
						seenName = true
						return false
					}
					val = p
					return true
				})
				if returnEscapesT(val, paramsByT) {
					found = true
					return true
				}
			case wrapperchecker.KindShorthandPropertyAssignment:
				// `{ x }` — the property value is the local `x`.
				c.ForEachChild(func(id *wrapperchecker.Node) bool {
					if id.Kind() == wrapperchecker.KindIdentifier &&
						paramsByT[id.LiteralText()] {
						found = true
						return true
					}
					return false
				})
				if found {
					return true
				}
			case wrapperchecker.KindSpreadAssignment:
				if returnEscapesT(c.FirstChild(), paramsByT) {
					found = true
					return true
				}
			}
			return false
		})
		return found
	case wrapperchecker.KindArrayLiteralExpression:
		found := false
		e.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindSpreadElement {
				if returnEscapesT(c.FirstChild(), paramsByT) {
					found = true
					return true
				}
				return false
			}
			if returnEscapesT(c, paramsByT) {
				found = true
				return true
			}
			return false
		})
		return found
	}
	return false
}

// parameterIdentifier returns the binding name of a Parameter node.
func parameterIdentifier(p *wrapperchecker.Node) string {
	var name string
	p.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
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
