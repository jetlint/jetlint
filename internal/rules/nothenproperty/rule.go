// Package nothenproperty implements no-then-property: an object that
// happens to expose `then` will be mistaken for a Promise by
// `Promise.resolve` / `await`, leading to surprising behavior. Avoid
// the name.
package nothenproperty

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-then-property"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAssignment:        visit,
		wrapperchecker.KindShorthandPropertyAssignment: visit,
		wrapperchecker.KindMethodDeclaration:         visit,
		wrapperchecker.KindGetAccessor:               visit,
		wrapperchecker.KindSetAccessor:               visit,
		wrapperchecker.KindPropertyDeclaration:       visit,
		wrapperchecker.KindPropertySignature:         visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	name := keyName(n)
	if name == "then" {
		ctx.Report(n, "`then` property — will be confused with a thenable by Promise machinery")
	}
}

func keyName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil {
		return ""
	}
	switch first.Kind() {
	case wrapperchecker.KindIdentifier:
		return first.SourceText()
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		s := first.LiteralText()
		return s
	case wrapperchecker.KindComputedPropertyName:
		var inner *wrapperchecker.Node
		first.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner == nil {
			return ""
		}
		switch inner.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return inner.LiteralText()
		}
	}
	return ""
}
