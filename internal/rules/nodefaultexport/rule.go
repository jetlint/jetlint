// Package nodefaultexport implements no-default-export: default
// exports break grep-ability and rename refactors. Named exports keep
// the canonical identifier discoverable.
package nodefaultexport

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-default-export"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindExportAssignment:    visitExportAssignment,
		wrapperchecker.KindExportDeclaration:   visitExportDeclaration,
		wrapperchecker.KindFunctionDeclaration: visitDecl,
		wrapperchecker.KindClassDeclaration:    visitDecl,
		wrapperchecker.KindInterfaceDeclaration: visitDecl,
	}
}

func visitDecl(ctx *engine.Context, n *wrapperchecker.Node) {
	if hasDefaultModifier(n) {
		ctx.Report(n, "default exports break refactor and grep — use a named export")
	}
}

func hasDefaultModifier(n *wrapperchecker.Node) bool {
	sawExport := false
	sawDefault := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindExportKeyword:
			sawExport = true
		case wrapperchecker.KindDefaultKeyword:
			sawDefault = true
		}
		return false
	})
	return sawExport && sawDefault
}

func visitExportAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	// `export default ...` parses to KindExportAssignment too in some
	// shapes; check source for "default".
	src := n.SourceText()
	if startsWithExportDefault(src) {
		ctx.Report(n, "default exports break refactor and grep — use a named export")
	}
}

func visitExportDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	if startsWithExportDefault(src) {
		ctx.Report(n, "default exports break refactor and grep — use a named export")
		return
	}
	// `export { foo as default }` and `export * as default from "x"`.
	hasDefaultRename := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindNamedExports || c.Kind() == wrapperchecker.KindNamespaceExport {
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() == wrapperchecker.KindExportSpecifier {
					// the "as default" piece is the alias child.
					var first, second *wrapperchecker.Node
					d.ForEachChild(func(e *wrapperchecker.Node) bool {
						if first == nil {
							first = e
						} else if second == nil {
							second = e
						}
						return false
					})
					alias := second
					if alias == nil {
						alias = first
					}
					if alias != nil {
						s := alias.SourceText()
						if s == "default" || s == "'default'" || s == `"default"` {
							hasDefaultRename = true
						}
					}
				}
				return false
			})
		}
		return false
	})
	if hasDefaultRename {
		ctx.Report(n, "renaming an export to `default` is still a default export — use a named export")
	}
}

func startsWithExportDefault(s string) bool {
	// Skip leading whitespace.
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	const prefix = "export default"
	if i+len(prefix) > len(s) {
		return false
	}
	if s[i:i+len(prefix)] != prefix {
		return false
	}
	// Must be followed by a non-identifier char so we don't match
	// `export defaults` (unlikely but safe).
	if i+len(prefix) == len(s) {
		return true
	}
	c := s[i+len(prefix)]
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ';' || c == '{' || c == '('
}
