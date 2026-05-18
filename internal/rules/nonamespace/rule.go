// Package nonamespace implements no-namespace: TS namespaces predate
// ES modules and the two don't play well together. Use modules
// instead. Ambient `declare module "name"` is exempt — that's a
// declaration file shape, not a code organization choice.
package nonamespace

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-namespace"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindModuleDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// `declare module "name"` declares an ambient module — allowed.
	// We detect by checking whether the name child is a string literal.
	src := strings.TrimSpace(n.SourceText())
	if strings.HasPrefix(src, "declare global") {
		return
	}
	// String-literal-named modules (ambient) — `declare module "foo"`,
	// `module "foo"` — allowed.
	if hasStringName(n) {
		return
	}
	ctx.Report(n, "TS namespaces predate ES modules — use a module instead")
}

func hasStringName(n *wrapperchecker.Node) bool {
	out := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStringLiteral {
			out = true
		}
		return false
	})
	return out
}
