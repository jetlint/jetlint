// Package preferreturnthistype implements the prefer-return-this-type
// rule: flag class methods declared with their own class as the
// return type that always return `this` — those should declare `this`
// as the return type so subclasses' chained calls keep their own type.
package preferreturnthistype

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-return-this-type"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindMethodDeclaration:   visit,
		wrapperchecker.KindPropertyDeclaration: visitProperty,
	}
}

// visitProperty handles `class C { f = (): C => { return this; }; }`.
func visitProperty(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	if parent.Kind() != wrapperchecker.KindClassDeclaration && parent.Kind() != wrapperchecker.KindClassExpression {
		return
	}
	className := classDeclaredName(parent)
	if className == "" {
		return
	}
	init := n.PropertyDeclarationInitializer()
	if init == nil {
		return
	}
	if init.Kind() != wrapperchecker.KindArrowFunction && init.Kind() != wrapperchecker.KindFunctionExpression {
		return
	}
	annot := init.FunctionReturnTypeAnnotation()
	if annot == nil {
		return
	}
	if typeAnnotationName(annot) != className {
		return
	}
	body := init.FunctionBody()
	if body == nil {
		return
	}
	if !methodAlwaysReturnsThis(body, init) {
		return
	}
	ctx.Report(init, "method always returns `this`; declare the return type as `this` so subclasses inherit chaining")
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	if parent.Kind() != wrapperchecker.KindClassDeclaration && parent.Kind() != wrapperchecker.KindClassExpression {
		return
	}
	className := classDeclaredName(parent)
	if className == "" {
		return
	}
	// `this:` parameter annotation explicitly types the receiver as a
	// concrete class — that's the developer opting out of polymorphic
	// `this`-typing. Don't second-guess.
	if hasThisParameterAnnotation(n) {
		return
	}
	annot := n.FunctionReturnTypeAnnotation()
	if annot == nil {
		return
	}
	annotIsUnion := annot.Kind() == wrapperchecker.KindUnionType
	annotName := typeAnnotationName(annot)
	if annotName != className {
		return
	}
	body := n.FunctionBody()
	if body == nil {
		return
	}
	// For a bare `: ClassName` annotation, every return path must be
	// `this` so swapping to `: this` doesn't change semantics. For a
	// union annotation containing the class name, only the
	// `ClassName` arm needs `this`-substitution; any return-this is
	// enough evidence that the arm is meaningful.
	if annotIsUnion {
		if !shouldFlagUnionReturn(ctx, parent, body, n) {
			return
		}
	} else if !methodAlwaysReturnsThis(body, n) {
		return
	}
	ctx.Report(n, "method always returns `this`; declare the return type as `this` so subclasses inherit chaining")
}

// shouldFlagUnionReturn applies the typescript-eslint algorithm for a
// union annotation containing the class name: flag iff at least one
// return is `this` AND no return has a type that is (or contains as a
// union member) the class type. The second clause keeps us from
// flagging cases like `f(): C | string { return this; return cu; }`
// where `cu: C | string` — the `C` arm there is reachable through
// `cu`, not just `this`, so swapping to `this | string` would lose
// the polymorphic info on the `C` side.
func shouldFlagUnionReturn(ctx *engine.Context, classNode, body, fn *wrapperchecker.Node) bool {
	classType := ctx.TypeOf(classNode)
	hasReturnThis := false
	hasReturnClassType := false
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil || hasReturnClassType {
			return
		}
		if n != fn {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration:
				return
			}
		}
		if n.Kind() == wrapperchecker.KindReturnStatement {
			expr := n.FirstChild()
			if expr == nil {
				return
			}
			if expr.Kind() == wrapperchecker.KindThisKeyword {
				hasReturnThis = true
				return
			}
			if returnExprIsThis(expr, body) {
				hasReturnThis = true
				return
			}
			if classType != nil {
				exprType := ctx.TypeOf(expr)
				if exprType != nil {
					if exprType.Equal(classType) {
						hasReturnClassType = true
						return
					}
					for _, m := range exprType.UnionMembers() {
						if m.Equal(classType) {
							hasReturnClassType = true
							return
						}
					}
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return hasReturnThis && !hasReturnClassType
}

// hasThisParameterAnnotation reports whether the method declares an
// explicit `this:` parameter — the first parameter named `this`.
func hasThisParameterAnnotation(fn *wrapperchecker.Node) bool {
	found := false
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindParameter {
			return false
		}
		c.ForEachChild(func(inner *wrapperchecker.Node) bool {
			if inner.Kind() == wrapperchecker.KindIdentifier && inner.LiteralText() == "this" {
				found = true
				return true
			}
			return false
		})
		return found
	})
	return found
}

// classDeclaredName returns the identifier name of a ClassDeclaration
// or ClassExpression. Empty for anonymous class expressions.
func classDeclaredName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// typeAnnotationName returns the identifier name of a TypeReference
// annotation (`Foo` in `: Foo`). For a union (`Foo | undefined`)
// returns the first member's name. Empty for anything else.
func typeAnnotationName(annot *wrapperchecker.Node) string {
	if annot == nil {
		return ""
	}
	if annot.Kind() == wrapperchecker.KindUnionType {
		var found string
		annot.ForEachChild(func(c *wrapperchecker.Node) bool {
			if name := typeAnnotationName(c); name != "" {
				found = name
				return true
			}
			return false
		})
		return found
	}
	var name string
	annot.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// returnExprIsThis reports whether the return expression carries the
// `this` reference — either directly (`return this`) or through a
// const-bound alias declared in the same body (`const self = this;
// return self;`).
func returnExprIsThis(expr *wrapperchecker.Node, body *wrapperchecker.Node) bool {
	if expr == nil {
		return false
	}
	if expr.Kind() == wrapperchecker.KindThisKeyword {
		return true
	}
	if expr.Kind() == wrapperchecker.KindIdentifier {
		name := expr.LiteralText()
		if name == "" {
			return false
		}
		return bodyDeclaresThisAlias(body, name)
	}
	return false
}

// bodyDeclaresThisAlias scans the function body for a top-level
// `const <name> = this;` declaration. Walks the immediate statements
// of the block; doesn't follow into nested blocks since reassignment
// inside conditionals would be hard to reason about.
func bodyDeclaresThisAlias(body *wrapperchecker.Node, name string) bool {
	if body == nil {
		return false
	}
	found := false
	body.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindVariableStatement {
			return false
		}
		stmt.ForEachChild(func(decllist *wrapperchecker.Node) bool {
			decllist.ForEachChild(func(decl *wrapperchecker.Node) bool {
				if decl.Kind() != wrapperchecker.KindVariableDeclaration {
					return false
				}
				init := decl.VariableDeclarationInitializer()
				if init == nil || init.Kind() != wrapperchecker.KindThisKeyword {
					return false
				}
				declName := variableDeclName(decl)
				if declName == name {
					found = true
				}
				return false
			})
			return false
		})
		return false
	})
	return found
}

func variableDeclName(decl *wrapperchecker.Node) string {
	var name string
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// methodAlwaysReturnsThis reports whether every reachable return
// statement returns the literal `this` keyword. Concise arrow bodies
// where the body itself is `this` also count.
func methodAlwaysReturnsThis(body *wrapperchecker.Node, fn *wrapperchecker.Node) bool {
	// Concise arrow body — the body's value is the return value.
	if body.Kind() != wrapperchecker.KindBlock {
		return body.Kind() == wrapperchecker.KindThisKeyword
	}
	hasReturn := false
	allThis := true
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n != fn {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration:
				return
			}
		}
		if n.Kind() == wrapperchecker.KindReturnStatement {
			hasReturn = true
			expr := n.FirstChild()
			if !returnExprIsThis(expr, body) {
				allThis = false
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return hasReturn && allThis
}

