// Package noconstructorreturn implements the no-constructor-return
// rule: a class constructor that returns a value from its body is
// almost always a mistake. Bare `return;` is fine — it exits the
// constructor early — but `return value;` either silently overrides
// the implicit `this` (when value is an object) or is ignored (when
// value is a primitive) and usually indicates the author confused
// constructors with factory functions.
package noconstructorreturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-constructor-return"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindReturnStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, ret *wrapperchecker.Node) {
	if !returnHasValue(ret) {
		return
	}
	if !isInsideConstructor(ret) {
		return
	}
	ctx.Report(ret, "Unexpected return statement in constructor.")
}

func returnHasValue(ret *wrapperchecker.Node) bool {
	has := false
	ret.ForEachChild(func(c *wrapperchecker.Node) bool {
		has = true
		return true
	})
	return has
}

// isInsideConstructor walks up from a return statement to the nearest
// function-like ancestor. It returns true only when that ancestor is a
// class constructor: nested functions/arrow expressions inside the
// constructor body have their own return scope and are not flagged.
func isInsideConstructor(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindConstructor:
			return true
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			return false
		}
	}
	return false
}
