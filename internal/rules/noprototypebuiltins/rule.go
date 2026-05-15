// Package noprototypebuiltins implements the no-prototype-builtins rule:
// calling `Object.prototype` methods (`hasOwnProperty`, `isPrototypeOf`,
// `propertyIsEnumerable`) directly on user objects is brittle.
// `Object.create(null)` produces objects without the prototype chain,
// and a hostile JSON payload can shadow these methods, so the canonical
// form is `Object.prototype.hasOwnProperty.call(obj, key)`.
package noprototypebuiltins

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-prototype-builtins"

// New constructs a noprototypebuiltins rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	for callee != nil && callee.Kind() == wrapperchecker.KindParenthesizedExpression {
		callee = callee.FirstChild()
	}
	name := staticPropertyName(callee)
	if name == "" {
		return
	}
	if !isProtoBuiltin(name) {
		return
	}
	ctx.Report(callee, fmt.Sprintf("do not access Object.prototype method %q from target object", name))
}

func isProtoBuiltin(name string) bool {
	switch name {
	case "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable":
		return true
	}
	return false
}

// staticPropertyName returns the literal property name when the callee
// is `obj.name`, `obj?.name`, `obj["name"]`, or `obj?.["name"]`. For
// computed accesses the index must be a string literal or a no-substitution
// template — anything else is dynamic and not subject to this rule.
func staticPropertyName(callee *wrapperchecker.Node) string {
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName()
	case wrapperchecker.KindElementAccessExpression:
		idx := callee.ElementAccessIndex()
		if idx == nil {
			return ""
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText()
		}
	}
	return ""
}
