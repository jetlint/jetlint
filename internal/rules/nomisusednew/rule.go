// Package nomisusednew implements the no-misused-new rule: classes
// must not define a body-less method named `new` whose return type is
// the enclosing class; interfaces must not define a construct signature
// whose return type is the enclosing interface; neither interfaces nor
// type literals may define a method signature named `constructor`.
//
// Pure-AST: no type lookups. The rule cross-references the declared
// name of the class/interface with the TypeReference name of the
// member's return type — `interface G { new <T>(): G<T>; }` still
// fires because the type reference name `G` matches the interface
// name even with type arguments.
package nomisusednew

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-misused-new"

// New constructs a no-misused-new rule.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindInterfaceDeclaration: visitInterface,
		wrapperchecker.KindClassDeclaration:     visitClass,
		wrapperchecker.KindClassExpression:      visitClass,
		wrapperchecker.KindMethodSignature:      visitMethodSignature,
	}
}

const interfaceMsg = "Interfaces cannot be constructed, only classes."
const classMsg = "Class cannot have method named `new`."

func visitInterface(ctx *engine.Context, n *wrapperchecker.Node) {
	name := declarationName(n)
	if name == "" {
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindConstructSignature {
			return false
		}
		if returnsTypeNamed(c, name) {
			ctx.Report(c, interfaceMsg)
		}
		return false
	})
}

func visitClass(ctx *engine.Context, n *wrapperchecker.Node) {
	name := declarationName(n)
	if name == "" {
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindMethodDeclaration {
			return false
		}
		if methodName(c) != "new" {
			return false
		}
		if c.FunctionBody() != nil {
			return false
		}
		if returnsTypeNamed(c, name) {
			ctx.Report(c, classMsg)
		}
		return false
	})
}

func visitMethodSignature(ctx *engine.Context, n *wrapperchecker.Node) {
	if methodName(n) == "constructor" {
		ctx.Report(n, interfaceMsg)
	}
}

// declarationName returns the textual name of a class- or interface-
// declaration node by walking its direct children for the first
// Identifier. Anonymous class expressions return "".
func declarationName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// methodName returns the textual name of a method-like member's first
// Identifier child. Computed property names (e.g. `[Symbol.iterator]`)
// return "" since their first child is not an Identifier.
func methodName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// returnsTypeNamed reports whether the function-like node's explicit
// return-type annotation is a TypeReference whose unqualified name
// equals the given string. Matches `Foo` and `Foo<T>` (type args are
// part of the TypeReference, not the type name).
func returnsTypeNamed(n *wrapperchecker.Node, name string) bool {
	ann := n.FunctionReturnTypeAnnotation()
	if ann == nil || ann.Kind() != wrapperchecker.KindTypeReference {
		return false
	}
	tn := ann.TypeReferenceTypeName()
	if tn == nil || tn.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	return tn.LiteralText() == name
}
