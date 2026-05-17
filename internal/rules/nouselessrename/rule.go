// Package nouselessrename implements no-useless-rename: `{ foo: foo }`
// and `import { foo as foo }` are no-ops — drop the rename.
package nouselessrename

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-rename"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBindingElement:    bindingElement,
		wrapperchecker.KindImportSpecifier:   importExportSpec,
		wrapperchecker.KindExportSpecifier:   importExportSpec,
	}
}

func bindingElement(ctx *engine.Context, n *wrapperchecker.Node) {
	// `{ foo: foo }` — first child is the property name, second the binding.
	var first, second *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 0 {
			first = c
		} else if idx == 1 {
			second = c
		}
		idx++
		return false
	})
	if first == nil || second == nil {
		return
	}
	// Only flag straight identifier → identifier with the same name.
	if first.Kind() != wrapperchecker.KindIdentifier || second.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if first.SourceText() == second.SourceText() {
		ctx.Report(n, "renaming `"+first.SourceText()+"` to itself is a no-op")
	}
}

func importExportSpec(ctx *engine.Context, n *wrapperchecker.Node) {
	// `{ foo as foo }`.
	var first, second *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 0 {
			first = c
		} else if idx == 1 {
			second = c
		}
		idx++
		return false
	})
	if first == nil || second == nil {
		return
	}
	if first.Kind() != wrapperchecker.KindIdentifier || second.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if first.SourceText() == second.SourceText() {
		ctx.Report(n, "renaming `"+first.SourceText()+"` to itself is a no-op")
	}
}
