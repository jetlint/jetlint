// Package nodynamicnamespaceimportaccess implements
// no-dynamic-namespace-import-access: flag computed property access
// against a namespace-imported binding, e.g.
//
//	import * as ns from "x";
//	ns["foo"];  // flagged
//	ns[key];    // flagged
//	ns.foo;     // allowed
//
// Computed access on the namespace defeats bundler tree-shaking and
// hides which exports the consumer actually depends on.
package nodynamicnamespaceimportaccess

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-dynamic-namespace-import-access"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	nsNames := collectNamespaceImportNames(src)
	if len(nsNames) == 0 {
		return
	}
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n.Kind() == wrapperchecker.KindElementAccessExpression {
			recv := n.ElementAccessReceiver()
			if recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && nsNames[recv.LiteralText()] {
				if !isAssignmentTarget(n) {
					ctx.Report(n, "computed access on a namespace import defeats tree-shaking — use a static property access")
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c)
		return false
	})
}

// isAssignmentTarget reports whether n appears on the left side of
// an assignment (`n = x`, `n += x`, etc.). Biome only flags reads.
func isAssignmentTarget(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	switch p.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
	default:
		return false
	}
	left := p.BinaryLeft()
	return left != nil && left.Pos() == n.Pos() && left.End() == n.End()
}

func collectNamespaceImportNames(src *wrapperchecker.Node) map[string]bool {
	out := map[string]bool{}
	src.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindImportDeclaration {
			return false
		}
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() != wrapperchecker.KindImportClause {
				return false
			}
			c.ForEachChild(func(cc *wrapperchecker.Node) bool {
				if cc.Kind() != wrapperchecker.KindNamespaceImport {
					return false
				}
				cc.ForEachChild(func(id *wrapperchecker.Node) bool {
					if id.Kind() == wrapperchecker.KindIdentifier {
						out[id.LiteralText()] = true
						return true
					}
					return false
				})
				return false
			})
			return false
		})
		return false
	})
	return out
}
