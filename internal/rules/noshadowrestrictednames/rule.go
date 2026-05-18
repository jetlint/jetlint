// Package noshadowrestrictednames implements
// no-shadow-restricted-names: declaring a local named `NaN`,
// `Infinity`, `undefined`, etc. shadows the global and creates a
// hidden surprise. Pick a different name.
package noshadowrestrictednames

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-shadow-restricted-names"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visitDecl,
		wrapperchecker.KindFunctionDeclaration: visitDecl,
		wrapperchecker.KindParameter:           visitDecl,
		wrapperchecker.KindClassDeclaration:    visitDecl,
		wrapperchecker.KindBindingElement:      visitDecl,
	}
}

var restricted = map[string]bool{
	"NaN": true, "Infinity": true, "undefined": true,
	"eval": true, "arguments": true,
	// Built-in globals worth protecting from local rebinding.
	"Array": true, "Object": true, "Number": true, "String": true, "Boolean": true,
	"Symbol": true, "BigInt": true, "Function": true, "RegExp": true, "Date": true,
	"Error": true, "JSON": true, "Math": true, "Map": true, "Set": true,
	"WeakMap": true, "WeakSet": true, "Promise": true, "Proxy": true, "Reflect": true,
	"globalThis": true,
}

func visitDecl(ctx *engine.Context, n *wrapperchecker.Node) {
	name := declName(n)
	if name == "" {
		return
	}
	if restricted[name] {
		ctx.Report(n, "don't shadow restricted name `"+name+"`")
	}
}

func declName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
		return first.SourceText()
	}
	return ""
}
