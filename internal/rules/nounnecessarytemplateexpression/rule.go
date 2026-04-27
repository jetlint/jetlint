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
		wrapperchecker.KindTemplateSpan: visitSpan,
	}
}

func visitSpan(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	// Skip tagged templates — the tag function decides what to do.
	if templateExpr := n.Parent(); templateExpr != nil {
		if outer := templateExpr.Parent(); outer != nil &&
			outer.Kind() == wrapperchecker.KindTaggedTemplateExpression {
			return
		}
	}
	if isLiteralExpression(expr) {
		ctx.Report(expr, "unnecessary template-literal interpolation of a literal value")
	}
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

