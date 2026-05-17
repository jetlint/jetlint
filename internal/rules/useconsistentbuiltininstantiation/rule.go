// Package useconsistentbuiltininstantiation implements
// use-consistent-builtin-instantiation: some built-ins are designed to
// be called with `new` (Object, Array, …), others without (Boolean,
// Number, …). Mixing them up changes the result type.
package useconsistentbuiltininstantiation

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-consistent-builtin-instantiation"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCall,
		wrapperchecker.KindNewExpression:  visitNew,
	}
}

// Constructors that must be invoked with `new`.
var needsNew = map[string]bool{
	"Object": true, "Array": true, "ArrayBuffer": true, "BigInt64Array": true,
	"BigUint64Array": true, "DataView": true, "Date": true, "Error": true,
	"Float32Array": true, "Float64Array": true, "Function": true,
	"Int8Array": true, "Int16Array": true, "Int32Array": true, "Map": true,
	"WeakMap": true, "Set": true, "WeakSet": true, "Promise": true,
	"RegExp": true, "Uint8Array": true, "Uint16Array": true, "Uint32Array": true,
	"Uint8ClampedArray": true, "SharedArrayBuffer": true, "Proxy": true,
	"WeakRef": true, "FinalizationRegistry": true,
}

// Functions that must NOT be invoked with `new`.
var noNew = map[string]bool{
	"Boolean": true, "Number": true, "String": true, "BigInt": true,
	"Symbol": true,
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	name := globalName(firstChild(n))
	if name == "" {
		return
	}
	if needsNew[name] && !isShadowed(n, name) {
		ctx.Report(n, "call `"+name+"` with `new`")
	}
}

func visitNew(ctx *engine.Context, n *wrapperchecker.Node) {
	name := globalName(firstChild(n))
	if name == "" {
		return
	}
	if noNew[name] && !isShadowed(n, name) {
		ctx.Report(n, "call `"+name+"` without `new`")
	}
}

func globalName(callee *wrapperchecker.Node) string {
	if callee == nil {
		return ""
	}
	if callee.Kind() == wrapperchecker.KindIdentifier {
		return callee.SourceText()
	}
	if callee.Kind() == wrapperchecker.KindPropertyAccessExpression {
		var obj, prop *wrapperchecker.Node
		callee.ForEachChild(func(c *wrapperchecker.Node) bool {
			if obj == nil {
				obj = c
			} else if prop == nil {
				prop = c
			}
			return false
		})
		if obj == nil || prop == nil || obj.Kind() != wrapperchecker.KindIdentifier {
			return ""
		}
		objName := obj.SourceText()
		if objName != "window" && objName != "globalThis" {
			return ""
		}
		return prop.SourceText()
	}
	return ""
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

func isShadowed(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		shadowed := false
		// Direct top-level scan.
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
		// For function scopes, also walk descendants for `var` (which hoists).
		if isFunctionLike(p) {
			if hasVarNamed(p, name) {
				return true
			}
		}
	}
	return false
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor:
		return true
	}
	return false
}

// hasVarNamed returns true when any `var name = ...` exists somewhere
// in n's body. `var` hoists to the enclosing function, so we don't need
// to track block boundaries — but we MUST stop at nested function
// boundaries.
func hasVarNamed(n *wrapperchecker.Node, name string) bool {
	if n == nil {
		return false
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if isFunctionLike(c) {
			return false
		}
		if c.Kind() == wrapperchecker.KindVariableStatement {
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() == wrapperchecker.KindVariableDeclarationList {
					// Only `var` (not let/const) hoists.
					if !isVarList(d) {
						return false
					}
					d.ForEachChild(func(decl *wrapperchecker.Node) bool {
						var first *wrapperchecker.Node
						decl.ForEachChild(func(x *wrapperchecker.Node) bool {
							if first == nil {
								first = x
							}
							return false
						})
						if first != nil && first.Kind() == wrapperchecker.KindIdentifier && first.SourceText() == name {
							found = true
						}
						return false
					})
				}
				return false
			})
			if found {
				return true
			}
		}
		if hasVarNamed(c, name) {
			found = true
			return true
		}
		return false
	})
	return found
}

func isVarList(n *wrapperchecker.Node) bool {
	src := n.SourceText()
	for i := 0; i < len(src)-3; i++ {
		if src[i] == 'v' && src[i+1] == 'a' && src[i+2] == 'r' {
			return true
		}
		if src[i] != ' ' && src[i] != '\t' && src[i] != '\n' {
			break
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
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindClassDeclaration:
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
