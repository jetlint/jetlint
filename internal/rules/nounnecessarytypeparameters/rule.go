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
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unnecessary-type-parameters"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration:  visit,
		wrapperchecker.KindFunctionExpression:   visit,
		wrapperchecker.KindArrowFunction:        visit,
		wrapperchecker.KindMethodDeclaration:    visit,
		wrapperchecker.KindMethodSignature:      visit,
		wrapperchecker.KindCallSignature:        visit,
		wrapperchecker.KindConstructSignature:   visit,
		wrapperchecker.KindFunctionType:         visit,
		wrapperchecker.KindConstructorType:      visit,
		wrapperchecker.KindClassDeclaration:     visitClass,
		wrapperchecker.KindClassExpression:      visitClass,
		wrapperchecker.KindInterfaceDeclaration: visitClass,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	tparams := typeParametersOf(n)
	if len(tparams) == 0 {
		return
	}
	// Only fall back to type-checker-based detection when the function
	// has no explicit return-type annotation. With an annotation, the
	// AST walk has already counted every reference; bumping via the
	// inferred type would double-count the explicit T and let
	// genuinely single-use parameters slip through.
	hasExplicitReturn := returnTypeAnnotation(n) != nil
	var rtMentions map[string]bool
	rtCheck := func(name string) bool {
		if hasExplicitReturn {
			return false
		}
		if rtMentions == nil {
			rtMentions = map[string]bool{}
			sig := ctx.Checker().SignatureOfDeclaration(n)
			if sig != nil {
				if rt := sig.ReturnType(); rt != nil {
					for _, tp := range tparams {
						pn := typeParameterName(tp)
						if pn == "" {
							continue
						}
						seen := map[string]bool{}
						if typeMentionsName(rt, pn, seen) {
							rtMentions[pn] = true
						}
					}
				}
			}
		}
		return rtMentions[name]
	}
	for _, tp := range tparams {
		name := typeParameterName(tp)
		if name == "" {
			continue
		}
		uses := countTypeParameterUsesInSignature(n, tp, name)
		if uses <= 1 && rtCheck(name) {
			uses++
		}
		if uses <= 1 {
			uses += countTypeParameterUsesInBodySignatures(n, name)
		}
		if uses > 1 {
			continue
		}
		ctx.Report(tp, "type parameter `"+name+"` is used only once — replace usages with its constraint or `unknown`")
	}
}

// visitClass checks the type parameters of a class/interface
// declaration. A class-level T is necessary iff it appears in at
// least one member's signature (parameter/return annotation, member
// annotation, heritage clause, or sibling type-parameter constraint).
// Single-use class type parameters carry no relation between members
// and are equivalent to `unknown` / their constraint.
func visitClass(ctx *engine.Context, n *wrapperchecker.Node) {
	tparams := typeParametersOf(n)
	if len(tparams) == 0 {
		return
	}
	for _, tp := range tparams {
		name := typeParameterName(tp)
		if name == "" {
			continue
		}
		uses := countTypeParameterUsesInClass(n, tp, name)
		if uses > 1 {
			continue
		}
		ctx.Report(tp, "type parameter `"+name+"` is used only once — replace usages with its constraint or `unknown`")
	}
}

// countTypeParameterUsesInClass counts identifier references with the
// given name across the class's heritage clauses, sibling type-
// parameter constraints/defaults, and every member's parameter and
// return-type annotations. Method bodies are not visited — only the
// declared interfaces of each member contribute.
func countTypeParameterUsesInClass(cls, owner *wrapperchecker.Node, name string) int {
	count := 0
	// walkType walks a type expression. `nested` is true when we are
	// inside a container (ArrayType, TupleType, TypeReference type
	// args, etc.) — upstream treats a class T appearing inside such a
	// container as "multiple uses", since the class T flows through
	// reads and writes via the wrapping generic. A bare `el: T`
	// appears as a top-level TypeReference and counts once.
	var walkType func(n *wrapperchecker.Node, nested bool)
	walkType = func(n *wrapperchecker.Node, nested bool) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier && n.LiteralText() == name {
			if nested {
				count += 2
			} else {
				count++
			}
			return
		}
		// Once we descend into a container kind, mark children as nested.
		childNested := nested
		switch n.Kind() {
		case wrapperchecker.KindArrayType,
			wrapperchecker.KindTupleType,
			wrapperchecker.KindUnionType,
			wrapperchecker.KindIntersectionType,
			wrapperchecker.KindIndexedAccessType,
			wrapperchecker.KindMappedType,
			wrapperchecker.KindConditionalType,
			wrapperchecker.KindTypeReference,
			wrapperchecker.KindTypeQuery,
			wrapperchecker.KindTypeLiteral,
			wrapperchecker.KindFunctionType,
			wrapperchecker.KindConstructorType,
			wrapperchecker.KindParenthesizedType,
			wrapperchecker.KindTypeOperator,
			wrapperchecker.KindRestType,
			wrapperchecker.KindOptionalType:
			childNested = true
		}
		// Special handling for TypeReference: the head identifier (name)
		// is the "use" itself; only mark type-argument children as nested.
		if n.Kind() == wrapperchecker.KindTypeReference {
			first := true
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if first {
					first = false
					walkType(c, nested)
				} else {
					walkType(c, true)
				}
				return false
			})
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walkType(c, childNested)
			return false
		})
	}
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindTypeParameter:
			if c == owner {
				return false
			}
			// Sibling constraint/default — skip the head identifier
			// (the param name) and walk the rest.
			first := true
			c.ForEachChild(func(part *wrapperchecker.Node) bool {
				if first && part.Kind() == wrapperchecker.KindIdentifier {
					first = false
					return false
				}
				first = false
				walkType(part, false)
				return false
			})
		case wrapperchecker.KindHeritageClause:
			// Heritage like `extends Foo<T>` — T inside type args is a use.
			walkType(c, false)
		case wrapperchecker.KindPropertyDeclaration,
			wrapperchecker.KindPropertySignature:
			// `name: T` — class property is both read+write; treat any
			// T appearance here as multiple uses (mirrors upstream
			// `assumeMultipleUses=true` for class members).
			c.ForEachChild(func(part *wrapperchecker.Node) bool {
				if part.Kind() != wrapperchecker.KindIdentifier &&
					part.Kind() != wrapperchecker.KindPrivateIdentifier {
					walkType(part, true)
				}
				return false
			})
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindMethodSignature,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor,
			wrapperchecker.KindCallSignature,
			wrapperchecker.KindConstructSignature,
			wrapperchecker.KindIndexSignature:
			// Member's own type parameters shadow the outer if same
			// name — but to keep the check simple, count any
			// appearance. Parameter type annotations and the return
			// annotation contribute.
			c.ForEachChild(func(part *wrapperchecker.Node) bool {
				switch part.Kind() {
				case wrapperchecker.KindParameter:
					walkType(part, false)
				case wrapperchecker.KindBlock,
					wrapperchecker.KindIdentifier,
					wrapperchecker.KindPrivateIdentifier:
					// Method body / member name — skip.
				default:
					walkType(part, false)
				}
				return false
			})
		}
		return false
	})
	return count
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

// nodeShadowsTypeParameter reports whether n is a nested function-like
// type/expression whose own type-parameter list re-declares `name`.
// When true, identifiers inside n bind to the inner declaration, so
// counting them as uses of the outer T would be incorrect.
func nodeShadowsTypeParameter(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionType,
		wrapperchecker.KindConstructorType,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindMethodSignature,
		wrapperchecker.KindCallSignature,
		wrapperchecker.KindConstructSignature,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		// fall through
	default:
		return false
	}
	for _, tp := range typeParametersOf(n) {
		if typeParameterName(tp) == name {
			return true
		}
	}
	return false
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
	var walk func(n *wrapperchecker.Node, inTypeQuery, inTypeRefArgs bool)
	walk = func(n *wrapperchecker.Node, inTypeQuery, inTypeRefArgs bool) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier {
			text := n.LiteralText()
			if text == name {
				if inTypeRefArgs {
					// T appearing as a type-argument of a non-Array/
					// Tuple generic (e.g. `ItemProps<T>`) is treated
					// by upstream as a multi-use reference.
					count += 2
				} else {
					count++
				}
				return
			}
			if inTypeQuery && paramsByT[text] {
				// `typeof param` where param: ...T... — semantic use.
				count++
				return
			}
		}
		// Stop recursing into a nested function-like that shadows our
		// type parameter name with its own declaration of the same name.
		if nodeShadowsTypeParameter(n, name) {
			return
		}
		isTQ := inTypeQuery || n.Kind() == wrapperchecker.KindTypeQuery
		// TypeReference: the head identifier doesn't put children in
		// type-arg context, but everything after the head does.
		if n.Kind() == wrapperchecker.KindTypeReference {
			first := true
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if first {
					first = false
					walk(c, isTQ, inTypeRefArgs)
				} else {
					walk(c, isTQ, true)
				}
				return false
			})
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c, isTQ, inTypeRefArgs)
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
		walk(annot, false, false)
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
			walk(part, false, false)
			return false
		})
		return false
	})
	if rt := returnTypeAnnotation(fn); rt != nil {
		preReturn := count
		walk(rt, false, false)
		// A return type that mentions T as a type argument (e.g.
		// `Set<T>`, `Promise<T | null>`) is the load-bearing case
		// upstream treats as valid even when T appears only once.
		// Bump by 2 so the "more than one use" gate skips.
		if count > preReturn && returnAnnotMentionsTAsTypeArg(rt, name) {
			count += 2
		}
	} else {
		// No explicit return type — check whether the body returns one
		// of the T-carrying parameters in a SHAPE-preserving way, or
		// explicitly references T inside a cast target / type argument
		// position. Both make T appear in the inferred return type.
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
					// Even when the return shape doesn't preserve T,
					// look at type-argument and cast positions inside
					// the return expression — those mentions make T
					// load-bearing in the inferred return type, mirroring
					// the upstream rule's "does removing T change the
					// signature?" check. Bump by 2 so the rule's "more
					// than one use" gate skips regardless of how many
					// times T is referenced.
					if !added {
						n.ForEachChild(func(c *wrapperchecker.Node) bool {
							if countExplicitTInTypePos(c, name) > 0 {
								count += 2
								added = true
							}
							return added
						})
					}
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

// returnContainsExplicitT reports whether the return-expression
// mentions the type parameter name in an explicit type position —
// a type argument (`new Map<K, V>()`, `fn<T>()`) or a cast target
// (`as T[]`, `<T>x`). Those positions produce an inferred return
// type that includes the type parameter even when the value-shape
// itself doesn't carry it.
func countExplicitTInTypePos(e *wrapperchecker.Node, name string) int {
	if e == nil {
		return 0
	}
	hits := 0
	var walk func(n *wrapperchecker.Node, inTypePos bool)
	walk = func(n *wrapperchecker.Node, inTypePos bool) {
		if n == nil {
			return
		}
		if inTypePos && n.Kind() == wrapperchecker.KindIdentifier {
			if n.LiteralText() == name {
				hits++
				return
			}
		}
		// Type-argument and cast-target subtrees enter type position.
		inSub := inTypePos
		switch n.Kind() {
		case wrapperchecker.KindAsExpression:
			walk(n.AsExpressionSource(), inTypePos)
			walk(n.AsExpressionTarget(), true)
			return
		case wrapperchecker.KindTypeAssertionExpression:
			walk(n.TypeAssertionSource(), inTypePos)
			walk(n.TypeAssertionTarget(), true)
			return
		case wrapperchecker.KindSatisfiesExpression:
			first := true
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if first {
					first = false
					walk(c, inTypePos)
				} else {
					walk(c, true)
				}
				return false
			})
			return
		case wrapperchecker.KindTypeReference,
			wrapperchecker.KindUnionType,
			wrapperchecker.KindIntersectionType,
			wrapperchecker.KindParenthesizedType,
			wrapperchecker.KindTypeQuery,
			wrapperchecker.KindFunctionType:
			inSub = true
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c, inSub)
			return false
		})
	}
	walk(e, false)
	return hits
}

// returnAnnotMentionsTAsTypeArg reports whether the return-type
// annotation mentions the type parameter name in a load-bearing
// position: as a type argument to another generic (`Set<T>`,
// `Promise<T | null>`) or as the element type of a non-readonly
// array (`T[]`). The presence of such a usage means the return
// type's shape depends on T — removing T would change the
// signature, so the parameter is load-bearing even when T appears
// textually only once. Bare `: T` and union/intersection/mapped
// shapes mentioning T do not qualify on their own.
func returnAnnotMentionsTAsTypeArg(rt *wrapperchecker.Node, name string) bool {
	if rt == nil {
		return false
	}
	found := false
	// containsName reports whether the subtree rooted at n contains a
	// reference to `name` (with shadowing respected).
	var containsName func(n *wrapperchecker.Node) bool
	containsName = func(n *wrapperchecker.Node) bool {
		if n == nil {
			return false
		}
		if n.Kind() == wrapperchecker.KindIdentifier && n.LiteralText() == name {
			return true
		}
		if nodeShadowsTypeParameter(n, name) {
			return false
		}
		out := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if containsName(c) {
				out = true
				return true
			}
			return false
		})
		return out
	}
	var top func(n *wrapperchecker.Node, underReadonly bool)
	top = func(n *wrapperchecker.Node, underReadonly bool) {
		if n == nil || found {
			return
		}
		if nodeShadowsTypeParameter(n, name) {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindTypeReference:
			// Skip the head identifier and scan each type argument.
			first := true
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if first {
					first = false
				} else if containsName(c) {
					found = true
					return true
				}
				return false
			})
			if found {
				return
			}
		case wrapperchecker.KindArrayType:
			// `T[]` element-type uses T — load-bearing in return.
			// `readonly T[]` (TypeOperator readonly → ArrayType) does
			// not count; upstream treats ReadonlyArray as a single use.
			if !underReadonly && containsName(n) {
				found = true
				return
			}
		case wrapperchecker.KindTypeOperator:
			if n.TypeOperatorOperator() == wrapperchecker.KindReadonlyKeyword {
				n.ForEachChild(func(c *wrapperchecker.Node) bool {
					top(c, true)
					return found
				})
				return
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			top(c, underReadonly)
			return found
		})
	}
	top(rt, false)
	return found
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
	case wrapperchecker.KindElementAccessExpression:
		// `obj[key]` produces `obj_T[key_T]` — an indexed access type
		// that preserves both T parameters in the inferred return.
		// Either the receiver or the index expression carrying T is
		// enough.
		if returnEscapesT(e.ElementAccessReceiver(), paramsByT) {
			return true
		}
		if returnEscapesT(e.ElementAccessIndex(), paramsByT) {
			return true
		}
		return false
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
// of a function-like signature, or nil when inferred or absent.
// Delegates to the wrapper's FunctionReturnTypeAnnotation accessor,
// which reads the AST's dedicated return-type slot — that's
// significantly more reliable than guessing from child kinds, which
// confuses an arrow function's expression body for a return type.
func returnTypeAnnotation(fn *wrapperchecker.Node) *wrapperchecker.Node {
	if fn == nil {
		return nil
	}
	return fn.FunctionReturnTypeAnnotation()
}

// countTypeParameterUsesInBodySignatures walks the function body
// looking for nested function-like nodes that are themselves part of
// the *return value* — only those contribute to the outer's signature
// from the caller's perspective. Counts references to `name` in
// their parameter and return-type annotations; nested functions that
// re-declare the same type-parameter name shadow the outer
// declaration and are skipped wholesale.
//
// For arrow functions with an expression body, the body itself is
// the returned value, so we descend through it. For block-bodied
// functions, we descend only through `return X` statements.
func countTypeParameterUsesInBodySignatures(fn *wrapperchecker.Node, name string) int {
	if fn == nil {
		return 0
	}
	body := fn.FunctionBody()
	if body == nil {
		return 0
	}
	count := 0
	var descend func(n *wrapperchecker.Node)
	descend = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindFunctionType,
			wrapperchecker.KindConstructorType,
			wrapperchecker.KindCallSignature,
			wrapperchecker.KindMethodSignature,
			wrapperchecker.KindConstructSignature:
			if nestedShadowsName(n, name) {
				return
			}
			countInNestedSignatureAnnotations(n, name, &count)
			// Don't recurse into the inner function's body — only its
			// signature contributes to the outer return's shape.
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			descend(c)
			return false
		})
	}
	if body.Kind() == wrapperchecker.KindBlock {
		// Block-bodied function: only `return X` paths count.
		var walkBlock func(n *wrapperchecker.Node)
		walkBlock = func(n *wrapperchecker.Node) {
			if n == nil {
				return
			}
			if n.Kind() == wrapperchecker.KindReturnStatement {
				n.ForEachChild(func(c *wrapperchecker.Node) bool {
					descend(c)
					return false
				})
				return
			}
			// Don't descend into nested function definitions when
			// hunting for return statements — each nested function
			// owns its own returns.
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration:
				return
			}
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walkBlock(c)
				return false
			})
		}
		walkBlock(body)
	} else {
		// Expression-bodied arrow: the body IS the returned value.
		descend(body)
	}
	return count
}

// nestedShadowsName reports whether a function-like node declares a
// type parameter with the same name as `name` — in which case the
// outer parameter is shadowed and any reference inside the nested
// signature refers to the nested declaration.
func nestedShadowsName(fn *wrapperchecker.Node, name string) bool {
	for _, tp := range typeParametersOf(fn) {
		if typeParameterName(tp) == name {
			return true
		}
	}
	return false
}

// countInNestedSignatureAnnotations counts identifier references with
// the given name across the parameter annotations and return-type
// annotation of a nested function-like node.
func countInNestedSignatureAnnotations(fn *wrapperchecker.Node, name string, count *int) {
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			if annot := c.ParameterTypeAnnotation(); annot != nil {
				countIdentInTypeNode(annot, name, count)
			}
		}
		return false
	})
	if rt := fn.FunctionReturnTypeAnnotation(); rt != nil {
		countIdentInTypeNode(rt, name, count)
	}
}

// countIdentInTypeNode walks a TypeNode subtree and increments
// `*count` for every Identifier whose text matches `name`.
func countIdentInTypeNode(n *wrapperchecker.Node, name string, count *int) {
	if n == nil {
		return
	}
	if n.Kind() == wrapperchecker.KindIdentifier && n.LiteralText() == name {
		*count++
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		countIdentInTypeNode(c, name, count)
		return false
	})
}

// typeMentionsName walks a wrapped Type recursively and returns true
// once it finds a type-parameter whose declared identifier matches
// `name`. The `seen` map prevents infinite recursion through self-
// referential aliases (`type T = { [P in keyof T]: T }`). The String()
// representation is used for de-duplication: two distinct Type values
// that print the same name describe the same shape for our purposes.
func typeMentionsName(t *wrapperchecker.Type, name string, seen map[string]bool) bool {
	if t == nil {
		return false
	}
	key := t.String()
	if seen[key] {
		return false
	}
	seen[key] = true
	if t.IsTypeParameter() && t.SymbolName() == name {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeMentionsName(m, name, seen) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if typeMentionsName(m, name, seen) {
				return true
			}
		}
		return false
	}
	for _, a := range t.TypeArguments() {
		if typeMentionsName(a, name, seen) {
			return true
		}
	}
	// Falls back to a textual scan: instantiated mapped/conditional
	// types, template literal types, and operator types (`keyof X`)
	// don't always surface their substructure through our wrapper
	// APIs, but their String() form spells the type parameter out as
	// an identifier — bordered by characters that aren't part of an
	// identifier in any of those positions.
	return identifierAppearsInTypeString(key, name)
}

// identifierAppearsInTypeString reports whether `name` appears in
// `s` as a standalone identifier (not as a fragment of a larger
// identifier like "T" inside "Test"). Identifier characters are the
// usual ASCII letters/digits/underscore plus `$`; everything else
// counts as a boundary.
func identifierAppearsInTypeString(s, name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i+len(name) <= len(s); i++ {
		if s[i:i+len(name)] != name {
			continue
		}
		if i > 0 && isIdentChar(s[i-1]) {
			continue
		}
		if i+len(name) < len(s) && isIdentChar(s[i+len(name)]) {
			continue
		}
		return true
	}
	return false
}

func isIdentChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_' || b == '$':
		return true
	}
	return false
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
