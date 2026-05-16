// Package nostringcasemismatch implements no-string-case-mismatch:
// comparing the result of `.toUpperCase()` or `.toLowerCase()`
// against a string literal that contains the opposite case is
// always false, so the surrounding branch is dead code. The
// author almost certainly typed the wrong case on the constant.
//
// Scope: BinaryExpression equality/inequality (`==`, `===`, `!=`,
// `!==`) and SwitchStatement discriminants. Template literals are
// only checked when they have no substitutions — interpolations
// would make the runtime value undecidable.
package nostringcasemismatch

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-string-case-mismatch"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visitBinary,
		wrapperchecker.KindSwitchStatement:  visitSwitch,
	}
}

func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isEqualityOperator(n.BinaryOperatorKind()) {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if method, ok := caseConverter(left); ok {
		if !literalMatchesCase(right, method) {
			ctx.Report(n, "comparison with mismatched case is always false")
		}
		return
	}
	if method, ok := caseConverter(right); ok {
		if !literalMatchesCase(left, method) {
			ctx.Report(n, "comparison with mismatched case is always false")
		}
	}
}

func visitSwitch(ctx *engine.Context, n *wrapperchecker.Node) {
	disc := n.SwitchExpression()
	method, ok := caseConverter(disc)
	if !ok {
		return
	}
	// SwitchStatement's body is the case block; ForEachChild on the
	// statement visits the discriminant then the block. Walking the
	// block's children yields the CaseClause / DefaultClause nodes
	// in source order.
	n.ForEachChild(func(block *wrapperchecker.Node) bool {
		block.ForEachChild(func(clause *wrapperchecker.Node) bool {
			if clause.Kind() != wrapperchecker.KindCaseClause {
				return false
			}
			label := clause.CaseExpression()
			if !literalMatchesCase(label, method) {
				ctx.Report(label, "case label has mismatched case from switch discriminant")
			}
			return false
		})
		return false
	})
}

// caseConverter returns the method name ("toUpperCase" / "toLowerCase")
// if n is a `<recv>.toX()` call with no arguments. The receiver and
// the call shape don't matter beyond that.
func caseConverter(n *wrapperchecker.Node) (string, bool) {
	if n == nil || n.Kind() != wrapperchecker.KindCallExpression {
		return "", false
	}
	if len(n.CallArguments()) != 0 {
		return "", false
	}
	callee := stripParens(n.CalleeExpression())
	if callee == nil {
		return "", false
	}
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		name := callee.PropertyAccessName()
		if name == "toUpperCase" || name == "toLowerCase" {
			return name, true
		}
	case wrapperchecker.KindElementAccessExpression:
		idx := stripParens(elementAccessIndex(callee))
		if idx == nil {
			return "", false
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			name := idx.LiteralText()
			if name == "toUpperCase" || name == "toLowerCase" {
				return name, true
			}
		}
	}
	return "", false
}

// literalMatchesCase reports whether n is a string-shaped literal
// whose value contains only characters compatible with the case
// produced by method. Returns true (no mismatch) for any input that
// isn't a static literal — the runtime value isn't known here.
func literalMatchesCase(n *wrapperchecker.Node, method string) bool {
	n = stripParens(n)
	if n == nil {
		return true
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return valueMatchesCase(n.LiteralText(), method)
	}
	return true
}

func valueMatchesCase(s, method string) bool {
	switch method {
	case "toUpperCase":
		for i := 0; i < len(s); i++ {
			if s[i] >= 'a' && s[i] <= 'z' {
				return false
			}
		}
	case "toLowerCase":
		for i := 0; i < len(s); i++ {
			if s[i] >= 'A' && s[i] <= 'Z' {
				return false
			}
		}
	}
	return true
}

func isEqualityOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken:
		return true
	}
	return false
}

func elementAccessIndex(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
			return false
		}
		second = c
		return true
	})
	return second
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
