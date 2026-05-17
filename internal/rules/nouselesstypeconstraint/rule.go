// Package nouselesstypeconstraint implements no-useless-type-constraint:
// `<T extends any>` and `<T extends unknown>` constrain to the
// universal type — same as no constraint. Drop the `extends` clause.
package nouselesstypeconstraint

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-type-constraint"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTypeParameter: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	// Detect "extends any" / "extends unknown".
	if hasTrailingExtends(src, "any") || hasTrailingExtends(src, "unknown") {
		ctx.Report(n, "type constraint `extends "+extractExtendsType(src)+"` is a no-op")
	}
}

func hasTrailingExtends(src, t string) bool {
	return extractExtendsType(src) == t
}

func extractExtendsType(src string) string {
	_, rest, ok := strings.Cut(src, "extends")
	if !ok {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if before, _, ok := strings.Cut(rest, "="); ok {
		rest = strings.TrimSpace(before)
	}
	return rest
}
