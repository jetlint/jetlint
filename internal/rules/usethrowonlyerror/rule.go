// Package usethrowonlyerror implements use-throw-only-error: throw
// an Error instance (or a subclass). Strings, numbers, and plain
// objects lose stack traces and surprise every error-handling layer.
package usethrowonlyerror

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-throw-only-error"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindThrowStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var arg *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		arg = c
		return false
	})
	if arg == nil {
		return
	}
	switch arg.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindTemplateExpression,
		wrapperchecker.KindNumericLiteral, wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindTrueKeyword, wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindObjectLiteralExpression, wrapperchecker.KindArrayLiteralExpression:
		ctx.Report(n, "throw an Error instance — non-Error throws lose stack traces")
	}
}
