// Package useliteralkeys implements use-literal-keys: `a["foo"]` is
// the same as `a.foo` — prefer the dot form when the key is a valid
// identifier.
package useliteralkeys

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-literal-keys"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindElementAccessExpression: visitAccess,
		wrapperchecker.KindComputedPropertyName:    visitComputed,
	}
}

func visitAccess(ctx *engine.Context, n *wrapperchecker.Node) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return
	}
	key := literalKey(second)
	if key == "" || key == "__proto__" {
		return
	}
	if !isValidIdent(key) {
		return
	}
	ctx.Report(n, "use dot access: `."+key+"` instead of `[\""+key+"\"]`")
}

func visitComputed(ctx *engine.Context, n *wrapperchecker.Node) {
	var inner *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if inner == nil {
			inner = c
		}
		return false
	})
	if inner == nil {
		return
	}
	key := literalKey(inner)
	if key == "" || key == "__proto__" {
		return
	}
	if !isValidIdent(key) {
		return
	}
	ctx.Report(n, "use the literal key `"+key+"` instead of `[\""+key+"\"]`")
}

func literalKey(n *wrapperchecker.Node) string {
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral:
		return strings.Trim(n.SourceText(), `"'`+"`")
	case wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return strings.Trim(n.SourceText(), "`")
	}
	return ""
}

func isValidIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '$') {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '$') {
				return false
			}
		}
	}
	return true
}
