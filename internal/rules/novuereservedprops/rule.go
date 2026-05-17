// Package novuereservedprops implements no-vue-reserved-props: a
// Vue 3 component prop named `ref` or `key` collides with Vue's
// built-in attributes — Vue uses those at the template level for
// element refs and list keys, so a same-named prop is silently
// shadowed and never reaches the component. The rule flags any
// props declaration that includes one of those reserved names,
// whether it's spelled as a defineProps array/object, a
// defineProps generic type argument, or the `props` option of an
// `export default {...}` / `defineComponent({...})` /
// `createApp({...})` descriptor.
package novuereservedprops

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-vue-reserved-props"

var reservedNames = map[string]bool{
	"ref": true,
	"key": true,
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:          visitCall,
		wrapperchecker.KindObjectLiteralExpression: visitObject,
	}
}

func visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.LiteralText() != "defineProps" {
		return
	}
	if arg := firstCallArg(call); arg != nil {
		reportReservedInPropsValue(ctx, arg)
	}
	if typeArg := firstTypeArg(call); typeArg != nil {
		reportReservedInTypeNode(ctx, typeArg)
	}
}

func visitObject(ctx *engine.Context, obj *wrapperchecker.Node) {
	if !isVueComponentDescriptor(obj) {
		return
	}
	prop := findProperty(obj, "props")
	if prop == nil {
		return
	}
	value := propertyValue(prop)
	if value == nil {
		return
	}
	reportReservedInPropsValue(ctx, value)
}

func reportReservedInPropsValue(ctx *engine.Context, value *wrapperchecker.Node) {
	value = unwrapParens(value)
	if value == nil {
		return
	}
	switch value.Kind() {
	case wrapperchecker.KindArrayLiteralExpression:
		value.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindStringLiteral {
				if reservedNames[c.LiteralText()] {
					ctx.Report(c, "this Vue prop name is reserved by Vue itself and will be shadowed by the framework")
				}
			}
			return false
		})
	case wrapperchecker.KindObjectLiteralExpression:
		value.ForEachChild(func(c *wrapperchecker.Node) bool {
			name := propertyKeyName(c)
			if name != "" && reservedNames[name] {
				ctx.Report(c, "this Vue prop name is reserved by Vue itself and will be shadowed by the framework")
			}
			return false
		})
	}
}

func reportReservedInTypeNode(ctx *engine.Context, typeNode *wrapperchecker.Node) {
	switch typeNode.Kind() {
	case wrapperchecker.KindTypeLiteral:
		typeNode.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindPropertySignature {
				name := propertyKeyName(c)
				if name != "" && reservedNames[name] {
					ctx.Report(c, "this Vue prop name is reserved by Vue itself and will be shadowed by the framework")
				}
			}
			return false
		})
	case wrapperchecker.KindTypeReference:
		name := typeReferenceName(typeNode)
		if name == "" {
			return
		}
		decl := findTypeDeclaration(typeNode, name)
		if decl != nil {
			reportReservedInTypeBody(ctx, decl)
		}
	}
}

func reportReservedInTypeBody(ctx *engine.Context, decl *wrapperchecker.Node) {
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindTypeLiteral:
			reportReservedInTypeNode(ctx, c)
		case wrapperchecker.KindPropertySignature:
			name := propertyKeyName(c)
			if name != "" && reservedNames[name] {
				ctx.Report(c, "this Vue prop name is reserved by Vue itself and will be shadowed by the framework")
			}
		}
		return false
	})
}

// isVueComponentDescriptor mirrors the carveout in the
// no-vue-data-object-declaration rule: an object literal is a Vue
// component descriptor when it appears as `export default`, the
// first argument to `createApp`/`Vue.createApp`/`defineComponent`,
// or the first argument to `new Vue(...)`. We share the AST shape
// rather than the package because we only need our own narrow
// matcher here.
func isVueComponentDescriptor(obj *wrapperchecker.Node) bool {
	p := obj.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindExportAssignment:
		return true
	case wrapperchecker.KindCallExpression:
		callee := p.CalleeExpression()
		if callee == nil {
			return false
		}
		var name string
		switch callee.Kind() {
		case wrapperchecker.KindIdentifier:
			name = callee.LiteralText()
		case wrapperchecker.KindPropertyAccessExpression:
			name = callee.PropertyAccessName()
		}
		if name != "createApp" && name != "defineComponent" {
			return false
		}
		// defineComponent supports `defineComponent(setupFn, options)`
		// where the descriptor object is the second argument; accept
		// either position for that call form.
		if nodeIsFirstArg(p, obj) {
			return true
		}
		if name == "defineComponent" && nodeIsAnyArg(p, obj) {
			return true
		}
		return false
	case wrapperchecker.KindNewExpression:
		if !nodeIsFirstArg(p, obj) {
			return false
		}
		callee := p.CalleeExpression()
		if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier {
			return callee.LiteralText() == "Vue"
		}
	}
	return false
}

func nodeIsAnyArg(call, arg *wrapperchecker.Node) bool {
	seenCallee := false
	var hit bool
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenCallee {
			seenCallee = true
			return false
		}
		inner := c
		for inner.Kind() == wrapperchecker.KindParenthesizedExpression {
			var x *wrapperchecker.Node
			inner.ForEachChild(func(g *wrapperchecker.Node) bool {
				x = g
				return true
			})
			if x == nil {
				break
			}
			inner = x
		}
		if inner.Pos() == arg.Pos() && inner.End() == arg.End() {
			hit = true
			return true
		}
		return false
	})
	return hit
}

func nodeIsFirstArg(call, arg *wrapperchecker.Node) bool {
	seenCallee := false
	var first *wrapperchecker.Node
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenCallee {
			seenCallee = true
			return false
		}
		if first == nil {
			first = c
			return true
		}
		return false
	})
	if first == nil {
		return false
	}
	for first.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		first.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner == nil {
			break
		}
		first = inner
	}
	return first.Pos() == arg.Pos() && first.End() == arg.End()
}

func findProperty(obj *wrapperchecker.Node, name string) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	obj.ForEachChild(func(c *wrapperchecker.Node) bool {
		if propertyKeyName(c) == name {
			found = c
			return true
		}
		return false
	})
	return found
}

func propertyKeyName(prop *wrapperchecker.Node) string {
	var name string
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			name = c.LiteralText()
			return true
		case wrapperchecker.KindStringLiteral:
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func propertyValue(prop *wrapperchecker.Node) *wrapperchecker.Node {
	if prop.Kind() != wrapperchecker.KindPropertyAssignment {
		return nil
	}
	seenName := false
	var value *wrapperchecker.Node
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName {
			if c.Kind() == wrapperchecker.KindIdentifier || c.Kind() == wrapperchecker.KindStringLiteral {
				seenName = true
			}
			return false
		}
		value = c
		return true
	})
	return value
}

func unwrapParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner == nil {
			break
		}
		n = inner
	}
	return n
}

func firstCallArg(call *wrapperchecker.Node) *wrapperchecker.Node {
	seenCallee := false
	var first *wrapperchecker.Node
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenCallee {
			seenCallee = true
			return false
		}
		if first == nil {
			first = c
			return true
		}
		return false
	})
	return first
}

func firstTypeArg(call *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindTypeLiteral, wrapperchecker.KindTypeReference:
			if first == nil {
				first = c
				return true
			}
		}
		return false
	})
	return first
}

func typeReferenceName(typeRef *wrapperchecker.Node) string {
	var name string
	typeRef.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func findTypeDeclaration(anchor *wrapperchecker.Node, name string) *wrapperchecker.Node {
	root := anchor
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return nil
	}
	return scanForTypeDeclaration(root, name)
}

func scanForTypeDeclaration(n *wrapperchecker.Node, name string) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindInterfaceDeclaration, wrapperchecker.KindTypeAliasDeclaration:
			if declarationName(c) == name {
				found = c
				return true
			}
		}
		if r := scanForTypeDeclaration(c, name); r != nil {
			found = r
			return true
		}
		return false
	})
	return found
}

func declarationName(d *wrapperchecker.Node) string {
	var name string
	d.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
