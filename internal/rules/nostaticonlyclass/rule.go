// Package nostaticonlyclass implements no-static-only-class: a class
// whose members are all static is just a namespace pretending to be a
// type. Use a module of free functions instead.
package nostaticonlyclass

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-static-only-class"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: visit,
		wrapperchecker.KindClassExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip if the class extends or implements — inheritance signals intent
	// beyond namespacing. Decorators count too.
	if strings.HasPrefix(strings.TrimSpace(n.SourceText()), "@") {
		return
	}
	memberCount := 0
	staticCount := 0
	hasHeritage := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindHeritageClause:
			hasHeritage = true
		case wrapperchecker.KindMethodDeclaration, wrapperchecker.KindPropertyDeclaration,
			wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			memberCount++
			if c.Kind() == wrapperchecker.KindConstructor {
				// A real constructor means the class is meant to be instantiated.
				memberCount = -1
				return true
			}
			if hasStaticModifier(c) {
				staticCount++
			}
		}
		return false
	})
	if hasHeritage || memberCount <= 0 || staticCount != memberCount {
		return
	}
	ctx.Report(n, "class with only static members — use a module of free functions instead")
}

func hasStaticModifier(n *wrapperchecker.Node) bool {
	out := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStaticKeyword {
			out = true
		}
		return false
	})
	return out
}
