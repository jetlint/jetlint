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
	annot := n.FunctionReturnTypeAnnotation()
	if annot == nil {
		return
	}
	annotName := typeAnnotationName(annot)
	if annotName != className {
		return
	}
	body := n.FunctionBody()
	if body == nil {
		return
	}
	if !methodAlwaysReturnsThis(body, n) {
		return
	}
	ctx.Report(n, "method always returns `this`; declare the return type as `this` so subclasses inherit chaining")
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
// annotation (`Foo` in `: Foo`). Empty for anything else.
func typeAnnotationName(annot *wrapperchecker.Node) string {
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
			if expr == nil || expr.Kind() != wrapperchecker.KindThisKeyword {
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
