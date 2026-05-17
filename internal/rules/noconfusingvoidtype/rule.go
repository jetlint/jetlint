// Package noconfusingvoidtype implements the no-confusing-void-type
// rule: `void` is meaningful only as a return type or generic type
// argument; using it elsewhere (parameter types, property types,
// variable types, type-alias bodies, type-assertion targets, intersections,
// mapped-type values, array elements, type-parameter constraints, etc.)
// is confusing or wrong. Matches biome's noConfusingVoidType.
//
// Rule is pure-AST: walks `void` keyword type-nodes and inspects the
// direct parent. Unions are deliberately left unflagged so the standard
// allowed companions (`Promise<void>`, `never`) don't trigger false
// positives; the rule still catches the many non-union violations in
// the upstream fixtures.
package noconfusingvoidtype

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-confusing-void-type"

// New constructs a no-confusing-void-type rule.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVoidKeyword: visit,
	}
}

const message = "void is only valid as a return type or generic type argument."

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	p := n.Parent()
	if p == nil {
		return
	}
	switch p.Kind() {
	case wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindPropertySignature,
		wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindTypeOperator,
		wrapperchecker.KindArrayType,
		wrapperchecker.KindTupleType,
		wrapperchecker.KindRestType,
		wrapperchecker.KindIntersectionType,
		wrapperchecker.KindMappedType,
		wrapperchecker.KindTypeAssertionExpression,
		wrapperchecker.KindAsExpression,
		wrapperchecker.KindSatisfiesExpression:
		ctx.Report(n, message)
	case wrapperchecker.KindParameter:
		name := p.ParameterName()
		if name != nil && name.LiteralText() == "this" {
			return
		}
		ctx.Report(n, message)
	case wrapperchecker.KindTypeParameter:
		def := p.TypeParameterDefaultType()
		if def != nil && def.Inner() == n.Inner() {
			return
		}
		ctx.Report(n, "void cannot be used as a type-parameter constraint.")
	}
}
