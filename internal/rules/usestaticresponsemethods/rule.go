// Package usestaticresponsemethods implements use-static-response-methods:
// `new Response(JSON.stringify(x))` is `Response.json(x)`; redirects
// are `Response.redirect(url, status)`.
package usestaticresponsemethods

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-static-response-methods"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNewExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.SourceText() != "Response" {
		return
	}
	if isShadowed(n, "Response") {
		return
	}
	args := callArgs(n)
	if len(args) == 0 {
		return
	}
	if !isJSONStringify(args[0]) {
		return
	}
	if isShadowed(n, "JSON") {
		return
	}
	// If a second argument is present, allow only `{}`, `{headers: {}}`,
	// or `{headers: {'Content-Type': 'application/json'}}` (case-insensitive
	// for the content type itself).
	if len(args) >= 2 {
		if !isAllowedJSONOptions(args[1]) {
			return
		}
	}
	ctx.Report(n, "use `Response.json(x)` instead of `new Response(JSON.stringify(x))`")
}

func isAllowedJSONOptions(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	var properties []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		properties = append(properties, c)
		return false
	})
	if len(properties) == 0 {
		return true
	}
	if len(properties) > 1 {
		return false
	}
	prop := properties[0]
	if prop.Kind() != wrapperchecker.KindPropertyAssignment {
		return false
	}
	var key, value *wrapperchecker.Node
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		if key == nil {
			key = c
		} else if value == nil {
			value = c
		}
		return false
	})
	if key == nil || value == nil {
		return false
	}
	if key.SourceText() != "headers" {
		return false
	}
	// headers value must be {} or { 'Content-Type': 'application/json' } (single property).
	if value.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	var hp []*wrapperchecker.Node
	value.ForEachChild(func(c *wrapperchecker.Node) bool {
		hp = append(hp, c)
		return false
	})
	if len(hp) == 0 {
		return true
	}
	if len(hp) > 1 {
		return false
	}
	if hp[0].Kind() != wrapperchecker.KindPropertyAssignment {
		return false
	}
	var hk, hv *wrapperchecker.Node
	hp[0].ForEachChild(func(c *wrapperchecker.Node) bool {
		if hk == nil {
			hk = c
		} else if hv == nil {
			hv = c
		}
		return false
	})
	if hk == nil || hv == nil {
		return false
	}
	kt := unquote(hk.SourceText())
	if !equalCI(kt, "content-type") {
		return false
	}
	vt := unquote(hv.SourceText())
	return equalCI(vt, "application/json")
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'' || s[0] == '`') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func equalCI(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func isJSONStringify(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	var obj, prop *wrapperchecker.Node
	callee.ForEachChild(func(c *wrapperchecker.Node) bool {
		if obj == nil {
			obj = c
		} else if prop == nil {
			prop = c
		}
		return false
	})
	if obj == nil || prop == nil {
		return false
	}
	if obj.Kind() != wrapperchecker.KindIdentifier || obj.SourceText() != "JSON" {
		return false
	}
	if prop.SourceText() != "stringify" {
		return false
	}
	// stringify must have at least one argument and not include a replacer fn.
	args := callArgs(n)
	if len(args) == 0 {
		return false
	}
	if len(args) >= 2 {
		// Replacer can be a function or an array; either way the conversion
		// to Response.json doesn't preserve semantics.
		return false
	}
	return true
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}

func callArgs(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var args []*wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
			args = append(args, c)
		}
		idx++
		return false
	})
	return args
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
	case wrapperchecker.KindClassDeclaration, wrapperchecker.KindFunctionDeclaration:
		var first *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if first == nil && d.Kind() == wrapperchecker.KindIdentifier {
				first = d
			}
			return false
		})
		return first != nil && first.SourceText() == name
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
	}
	return false
}
