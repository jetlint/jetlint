// Package novuereservedkeys implements no-vue-reserved-keys: Vue
// reserves identifiers prefixed with `$` (instance API like `$el`,
// `$data`) on every section, and additionally reserves `_`-prefixed
// names on the `data`/`asyncData` return objects because Vue uses
// `_xxx` for internal bookkeeping there. Declaring one shadows the
// framework field. The rule flags the appropriate reserved prefix
// per Vue component section.
package novuereservedkeys

import (
	"os"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-vue-reserved-keys"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindObjectLiteralExpression: visitObject,
		wrapperchecker.KindCallExpression:          visitCall,
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
	if !inScriptSetup(call) {
		return
	}
	if arg := firstArg(call); arg != nil {
		checkPropsValue(ctx, arg)
	}
	if typeArg := firstTypeArg(call); typeArg != nil {
		checkTypeArg(ctx, typeArg)
	}
}

// inScriptSetup checks the file for the `@script-setup` marker
// emitted by the fixture extractor; defineProps is only a macro
// inside script-setup, otherwise it's a plain function call and
// the rule shouldn't fire. Without an SFC parser the marker is
// the cheapest reliable signal.
func inScriptSetup(n *wrapperchecker.Node) bool {
	root := n
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return false
	}
	if path, _, _, _, _ := root.SourceRange(); path != "" {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), "@script-setup") {
			return true
		}
	}
	return strings.Contains(root.SourceText(), "@script-setup")
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

func checkTypeArg(ctx *engine.Context, typeArg *wrapperchecker.Node) {
	switch typeArg.Kind() {
	case wrapperchecker.KindTypeLiteral:
		typeArg.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindPropertySignature {
				name := propertyKeyName(c)
				if name != "" && dollarOnly(name) {
					ctx.Report(c, "this Vue prop name is reserved by Vue itself")
				}
			}
			return false
		})
	case wrapperchecker.KindTypeReference:
		var name string
		typeArg.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier {
				name = c.LiteralText()
				return true
			}
			return false
		})
		if name == "" {
			return
		}
		decl := findTypeDeclByName(typeArg, name)
		if decl == nil {
			return
		}
		decl.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindTypeLiteral:
				checkTypeArg(ctx, c)
			case wrapperchecker.KindPropertySignature:
				name := propertyKeyName(c)
				if name != "" && dollarOnly(name) {
					ctx.Report(c, "this Vue prop name is reserved by Vue itself")
				}
			}
			return false
		})
	}
}

func findTypeDeclByName(anchor *wrapperchecker.Node, name string) *wrapperchecker.Node {
	root := anchor
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return nil
	}
	return scanForTypeDecl(root, name)
}

func scanForTypeDecl(n *wrapperchecker.Node, name string) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindInterfaceDeclaration, wrapperchecker.KindTypeAliasDeclaration:
			var nm string
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindIdentifier {
					nm = g.LiteralText()
					return true
				}
				return false
			})
			if nm == name {
				found = c
				return true
			}
		}
		if r := scanForTypeDecl(c, name); r != nil {
			found = r
			return true
		}
		return false
	})
	return found
}

func visitObject(ctx *engine.Context, obj *wrapperchecker.Node) {
	if !isVueComponentDescriptor(obj) {
		return
	}
	obj.ForEachChild(func(prop *wrapperchecker.Node) bool {
		switch propertyKeyName(prop) {
		case "props":
			if v := propertyValue(prop); v != nil {
				checkPropsValue(ctx, v)
			}
		case "data", "asyncData":
			if v := dataReturnedObject(prop); v != nil {
				checkObjectKeys(ctx, v, dollarOrUnderscore)
			}
		case "computed", "methods":
			if v := propertyValue(prop); v != nil {
				v = unwrap(v)
				if v != nil && v.Kind() == wrapperchecker.KindObjectLiteralExpression {
					checkObjectKeys(ctx, v, dollarOnly)
				}
			}
		case "setup":
			if v := dataReturnedObject(prop); v != nil {
				checkObjectKeys(ctx, v, dollarOnly)
			}
		}
		return false
	})
}

func checkPropsValue(ctx *engine.Context, v *wrapperchecker.Node) {
	v = unwrap(v)
	if v == nil {
		return
	}
	switch v.Kind() {
	case wrapperchecker.KindArrayLiteralExpression:
		v.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindStringLiteral {
				if dollarOnly(c.LiteralText()) {
					ctx.Report(c, "this Vue prop name is reserved by Vue itself")
				}
			}
			return false
		})
	case wrapperchecker.KindObjectLiteralExpression:
		checkObjectKeys(ctx, v, dollarOnly)
	}
}

func checkObjectKeys(ctx *engine.Context, obj *wrapperchecker.Node, reserved func(string) bool) {
	obj.ForEachChild(func(c *wrapperchecker.Node) bool {
		name := propertyKeyName(c)
		if name != "" && reserved(name) {
			ctx.Report(c, "this Vue key is reserved by Vue itself")
		}
		return false
	})
}

func dollarOnly(name string) bool {
	return strings.HasPrefix(name, "$")
}

func dollarOrUnderscore(name string) bool {
	return strings.HasPrefix(name, "$") || strings.HasPrefix(name, "_")
}

// dataReturnedObject extracts the object literal returned from a
// data-style property. The property's value can be a function (with
// `return {...}`), an arrow function with concise body `() => ({...})`
// or `() => {...}`, an arrow with paren-wrapped object literal, a
// shorthand method declaration `data() { return {...} }`, or a bare
// object literal.
func dataReturnedObject(prop *wrapperchecker.Node) *wrapperchecker.Node {
	if prop.Kind() == wrapperchecker.KindMethodDeclaration {
		return returnedObjectFromBlock(prop)
	}
	value := propertyValue(prop)
	if value == nil {
		return nil
	}
	value = unwrap(value)
	switch value.Kind() {
	case wrapperchecker.KindObjectLiteralExpression:
		return value
	case wrapperchecker.KindFunctionExpression:
		return returnedObjectFromBlock(value)
	case wrapperchecker.KindArrowFunction:
		body := arrowBody(value)
		if body == nil {
			return nil
		}
		body = unwrap(body)
		if body.Kind() == wrapperchecker.KindObjectLiteralExpression {
			return body
		}
		if body.Kind() == wrapperchecker.KindBlock {
			return returnedObjectFromBlock(value)
		}
	}
	return nil
}

// arrowBody returns the body node of an arrow function: the last
// direct child that is either a Block (block-bodied arrow) or an
// expression (concise body). The arrow token isn't exposed through
// the wrapper, so we just take the trailing structured child.
func arrowBody(arrow *wrapperchecker.Node) *wrapperchecker.Node {
	var last *wrapperchecker.Node
	arrow.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

func returnedObjectFromBlock(fnOrMethod *wrapperchecker.Node) *wrapperchecker.Node {
	var block *wrapperchecker.Node
	fnOrMethod.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			block = c
			return true
		}
		return false
	})
	if block == nil {
		return nil
	}
	var obj *wrapperchecker.Node
	block.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindReturnStatement {
			return false
		}
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			c = unwrap(c)
			if c != nil && c.Kind() == wrapperchecker.KindObjectLiteralExpression {
				obj = c
				return true
			}
			return false
		})
		return obj != nil
	})
	return obj
}

func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
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

func firstArg(call *wrapperchecker.Node) *wrapperchecker.Node {
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

// isVueComponentDescriptor mirrors the matcher used by other Vue
// rules: an object literal is treated as a Vue component descriptor
// when it appears as `export default ...`, the argument to
// `defineComponent`/`createApp`, or the argument to `new Vue(...)`.
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
		return name == "createApp" || name == "defineComponent"
	case wrapperchecker.KindNewExpression:
		callee := p.CalleeExpression()
		return callee != nil && callee.Kind() == wrapperchecker.KindIdentifier && callee.LiteralText() == "Vue"
	}
	return false
}
