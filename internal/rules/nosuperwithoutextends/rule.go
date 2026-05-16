// Package nosuperwithoutextends implements no-super-without-extends:
// `super(...)` and `super.x` only resolve to something useful when
// the enclosing class has an `extends` clause. Calling `super` in a
// non-derived class is a `SyntaxError` at runtime in modern engines
// — biome flags it ahead of time so the failure surfaces during
// development instead of at the first instantiation.
package nosuperwithoutextends

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-super-without-extends"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSuperKeyword: visit,
	}
}

func visit(ctx *engine.Context, super *wrapperchecker.Node) {
	class := enclosingClass(super)
	if class == nil {
		return
	}
	if classExtendsSomething(class) {
		return
	}
	ctx.Report(super, "`super` is only valid inside a class with an `extends` clause")
}

func enclosingClass(n *wrapperchecker.Node) *wrapperchecker.Node {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
			return p
		}
	}
	return nil
}

func classExtendsSomething(class *wrapperchecker.Node) bool {
	var hit bool
	class.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindHeritageClause {
			return false
		}
		if heritageIsExtends(c) {
			hit = true
			return true
		}
		return false
	})
	return hit
}

// heritageIsExtends distinguishes the `extends` clause from
// `implements`. Both arrive as HeritageClause nodes; the wrapper
// exposes the leading keyword as a Kind value via HeritageClauseToken.
func heritageIsExtends(clause *wrapperchecker.Node) bool {
	return clause.HeritageClauseToken() == wrapperchecker.KindExtendsKeyword
}
