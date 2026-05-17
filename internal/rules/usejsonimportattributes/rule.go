// Package usejsonimportattributes implements use-json-import-attributes:
// an `import` whose specifier ends in `.json` must declare a JSON
// import attribute (`with { type: 'json' }`). Without it, modern
// runtimes (Node ≥ 20.10, recent browsers) refuse the import for
// security reasons — the missing attribute is a silent shipping
// regression rather than a typo.
//
// Detection uses the import's source text: TypeScript's wrapper
// doesn't expose `KindImportAttributes` directly, so the rule
// inspects the textual `type: "json"` (or `'json'`) pattern inside
// the trailing `with { ... }` clause. That clause is the only
// place biome and the runtime look for the attribute.
package usejsonimportattributes

import (
	"regexp"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-json-import-attributes"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visit,
	}
}

// jsonTypeAttr matches `type` followed by `:` and a `'json'` or
// `"json"` string literal, ignoring whitespace and the surrounding
// `with { ... }` content. Tolerant of other attributes in the
// braces because the runtime only requires the type entry to be
// present and correct.
var jsonTypeAttr = regexp.MustCompile(`type\s*:\s*['"]json['"]`)

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	spec := n.ModuleSpecifier()
	if spec == nil || spec.Kind() != wrapperchecker.KindStringLiteral {
		return
	}
	path := spec.LiteralText()
	if !strings.HasSuffix(path, ".json") {
		return
	}
	src := n.SourceText()
	if jsonTypeAttr.MatchString(src) {
		return
	}
	ctx.Report(n, "add `with { type: 'json' }` — modern runtimes require JSON import attributes")
}
