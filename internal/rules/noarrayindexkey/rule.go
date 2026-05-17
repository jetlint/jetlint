// Package noarrayindexkey implements no-array-index-key: in a React
// list, `key={index}` defeats reconciliation when items shift — use a
// stable id.
package noarrayindexkey

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-array-index-key"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxAttribute: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var name, value *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if name == nil {
			name = c
		} else if value == nil {
			value = c
		}
		return false
	})
	if name == nil || value == nil {
		return
	}
	if name.Kind() != wrapperchecker.KindIdentifier || name.SourceText() != "key" {
		return
	}
	if referencesIndex(value) {
		ctx.Report(n, "don't use the array index as a React `key` — it changes when items reorder")
	}
}

// referencesIndex reports whether n contains an Identifier `index`
// in a value (read) position — not as a property name.
func referencesIndex(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindIdentifier && n.SourceText() == "index" {
		return true
	}
	// Skip the property-name child of PropertyAccessExpression.
	found := false
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		var first *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
			}
			return false
		})
		if referencesIndex(first) {
			return true
		}
		return false
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if referencesIndex(c) {
			found = true
			return true
		}
		return false
	})
	return found
}
