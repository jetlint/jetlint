// Package nouselessstringraw implements no-useless-string-raw:
// `String.raw` is for strings that contain real backslashes that
// shouldn't be interpreted as escapes. A `String.raw` template with
// no backslashes is just punctuation — drop it.
package nouselessstringraw

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-string-raw"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTaggedTemplateExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if first == nil || second == nil {
		return
	}
	if first.SourceText() != "String.raw" {
		return
	}
	// Inspect the template literal: if no backslashes appear in any raw
	// chunk, the tag is pointless.
	if !strings.Contains(second.SourceText(), `\`) {
		ctx.Report(n, "String.raw without any backslashes is a no-op — drop the tag")
	}
}
