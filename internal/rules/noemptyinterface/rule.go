// Package noemptyinterface implements no-empty-interface: an empty
// interface is a documentation alias for the type system but adds
// nothing — `type Foo = Bar` does the same job and reads honestly.
// Empty interfaces that extend another are still useful (declaration
// merging, semantic alias).
package noemptyinterface

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-empty-interface"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindInterfaceDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if insideAmbientModule(n) {
		return
	}
	hasMembers := false
	hasExtends := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindHeritageClause:
			hasExtends = true
		case wrapperchecker.KindPropertySignature, wrapperchecker.KindMethodSignature,
			wrapperchecker.KindIndexSignature, wrapperchecker.KindCallSignature,
			wrapperchecker.KindConstructSignature:
			hasMembers = true
		}
		return false
	})
	if hasMembers || hasExtends {
		return
	}
	ctx.Report(n, "empty interface — declare a type alias or remove it")
}

func insideAmbientModule(n *wrapperchecker.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindModuleDeclaration {
			return true
		}
	}
	return false
}
