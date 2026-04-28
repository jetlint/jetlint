// Package nounnecessarytemplateexpression implements the
// no-unnecessary-template-expression rule: flag template-literal
// interpolations of literal-typed expressions and single-interpolation
// templates that just stringify an already-string value.
package nounnecessarytemplateexpression

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-template-expression"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTemplateSpan:           visitSpan,
		wrapperchecker.KindTemplateLiteralTypeSpan: visitTypeSpan,
	}
}

func visitSpan(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	templateExpr := n.Parent()
	if templateExpr == nil {
		return
	}
	// Skip tagged templates — the tag function decides what to do.
	if outer := templateExpr.Parent(); outer != nil && outer.Kind() == wrapperchecker.KindTaggedTemplateExpression {
		return
	}
	if isLiteralExpression(expr) {
		ctx.Report(expr, "unnecessary template-literal interpolation of a literal value")
	}
}

// visitTypeSpan handles type-level template literals like `${1}`,
// `${null}`, `${undefined}`, `${'foo'}`. The interpolated type-node
// is the first child; when it's a literal type that's already a
// stringifiable singleton, the wrapping template adds nothing.
// Empty string literals are exempt — `${''}` is a deliberate
// whitespace marker upstream considers load-bearing.
func visitTypeSpan(ctx *engine.Context, n *wrapperchecker.Node) {
	tnode := n.FirstChild()
	if tnode == nil {
		return
	}
	if !isLiteralOrUndefinedTypeNode(tnode) {
		return
	}
	if isEmptyStringLiteralType(tnode) {
		return
	}
	ctx.Report(tnode, "unnecessary template-literal-type interpolation of a literal type")
}

// isLiteralOrUndefinedTypeNode reports whether the type-node `t` is a
// literal type, the `null` keyword, or the `undefined` keyword — the
// shapes whose interpolation in a template-literal type contributes
// no information beyond the literal itself.
func isLiteralOrUndefinedTypeNode(t *wrapperchecker.Node) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case wrapperchecker.KindLiteralType,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindUndefinedKeyword,
		wrapperchecker.KindTemplateLiteralType:
		return true
	}
	return false
}

// isEmptyStringLiteralType reports whether t is `LiteralType` wrapping
// an empty-string literal like `''` or `""`.
func isEmptyStringLiteralType(t *wrapperchecker.Node) bool {
	if t == nil || t.Kind() != wrapperchecker.KindLiteralType {
		return false
	}
	inner := t.FirstChild()
	if inner == nil {
		return false
	}
	switch inner.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return inner.LiteralText() == ""
	}
	return false
}

func isLiteralExpression(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword:
		return true
	}
	return false
}

