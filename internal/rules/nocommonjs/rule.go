// Package nocommonjs implements no-common-js: `require(...)`,
// `module.exports`, and `exports.X` are CommonJS — use ES module
// `import`/`export` instead.
package nocommonjs

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-common-js"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:             visitCall,
		wrapperchecker.KindPropertyAccessExpression:   visitAccess,
	}
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	var callee *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if callee == nil {
			callee = c
		}
		return false
	})
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.SourceText() != "require" {
		return
	}
	if isShadowed(n, "require") {
		return
	}
	ctx.Report(n, "use ES `import` instead of `require()`")
}

func visitAccess(ctx *engine.Context, n *wrapperchecker.Node) {
	var obj, prop *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if obj == nil {
			obj = c
		} else if prop == nil {
			prop = c
		}
		return false
	})
	if obj == nil || prop == nil || obj.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	// `module.exports`
	if obj.SourceText() == "module" && prop.SourceText() == "exports" && !isShadowed(n, "module") {
		ctx.Report(n, "use ES `export` instead of `module.exports`")
		return
	}
	// `exports.X`
	if obj.SourceText() == "exports" && !isShadowed(n, "exports") {
		ctx.Report(n, "use ES `export` instead of `exports.…`")
	}
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		shadowed := false
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if declaresName(c, name) {
				shadowed = true
				return true
			}
			return false
		})
		if shadowed {
			return true
		}
	}
	return false
}

func declaresName(c *wrapperchecker.Node, name string) bool {
	switch c.Kind() {
	case wrapperchecker.KindVariableStatement:
		hit := false
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if d.Kind() == wrapperchecker.KindVariableDeclarationList {
				d.ForEachChild(func(decl *wrapperchecker.Node) bool {
					var first *wrapperchecker.Node
					decl.ForEachChild(func(x *wrapperchecker.Node) bool {
						if first == nil {
							first = x
						}
						return false
					})
					if first != nil && first.Kind() == wrapperchecker.KindIdentifier && first.SourceText() == name {
						hit = true
					}
					return false
				})
			}
			return false
		})
		return hit
	case wrapperchecker.KindFunctionDeclaration:
		var first *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if first == nil && d.Kind() == wrapperchecker.KindIdentifier {
				first = d
			}
			return false
		})
		return first != nil && first.SourceText() == name
	}
	return false
}
