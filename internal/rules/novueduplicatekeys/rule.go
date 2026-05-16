// Package novueduplicatekeys implements no-vue-duplicate-keys: a Vue
// component must not declare the same name in two places where Vue
// will merge them into a single instance member, because the second
// silently shadows the first. The rule walks Vue component descriptors
// (`export default {...}`, `defineComponent({...})`, `createApp({...})`,
// `new Vue({...})`) collecting key names from props, computed, data,
// asyncData, methods, and setup's returned object. In `<script setup>`
// files the rule also collects top-level bindings and prop names from
// bare `defineProps` calls.
package novueduplicatekeys

import (
	"os"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-vue-duplicate-keys"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindObjectLiteralExpression: visitObject,
		wrapperchecker.KindSourceFile:              visitSourceFile,
	}
}

func visitObject(ctx *engine.Context, obj *wrapperchecker.Node) {
	if !isVueComponentDescriptor(obj) {
		return
	}
	var keys []*wrapperchecker.Node
	obj.ForEachChild(func(prop *wrapperchecker.Node) bool {
		switch propertyKeyName(prop) {
		case "props":
			keys = append(keys, collectPropsKeys(prop)...)
		case "computed", "methods":
			keys = append(keys, collectObjectKeys(propertyValueObject(prop))...)
		case "data", "asyncData":
			keys = append(keys, collectObjectKeys(dataReturnedObject(prop))...)
		case "setup":
			keys = append(keys, collectObjectKeys(dataReturnedObject(prop))...)
		}
		return false
	})
	reportDuplicates(ctx, keys)
}

func visitSourceFile(ctx *engine.Context, file *wrapperchecker.Node) {
	if !inScriptSetup(file) {
		return
	}
	var keys []*wrapperchecker.Node
	file.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		switch stmt.Kind() {
		case wrapperchecker.KindVariableStatement:
			keys = append(keys, collectVarStatementBindings(stmt)...)
		case wrapperchecker.KindExpressionStatement:
			keys = append(keys, collectBareDefinePropsKeys(stmt)...)
		case wrapperchecker.KindFunctionDeclaration:
			if name := functionDeclarationName(stmt); name != nil {
				keys = append(keys, name)
			}
		}
		return false
	})
	reportDuplicates(ctx, keys)
}

func reportDuplicates(ctx *engine.Context, keys []*wrapperchecker.Node) {
	if len(keys) < 2 {
		return
	}
	seen := map[string]bool{}
	for _, k := range keys {
		name := nodeText(k)
		if name == "" {
			continue
		}
		if seen[name] {
			ctx.Report(k, "Duplicate Vue key "+name)
			continue
		}
		seen[name] = true
	}
}

// nodeText returns the text of an Identifier / StringLiteral key node.
func nodeText(n *wrapperchecker.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case wrapperchecker.KindIdentifier, wrapperchecker.KindStringLiteral, wrapperchecker.KindPrivateIdentifier:
		return n.LiteralText()
	}
	return ""
}

func collectPropsKeys(prop *wrapperchecker.Node) []*wrapperchecker.Node {
	v := propertyValueAny(prop)
	if v == nil {
		return nil
	}
	v = unwrap(v)
	switch v.Kind() {
	case wrapperchecker.KindArrayLiteralExpression:
		var out []*wrapperchecker.Node
		v.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindStringLiteral {
				out = append(out, c)
			}
			return false
		})
		return out
	case wrapperchecker.KindObjectLiteralExpression:
		return collectObjectKeys(v)
	}
	return nil
}

// collectObjectKeys returns the key identifier/string-literal node for
// every literal property in obj. Spread elements (`...x`) have no key
// of their own — skipped explicitly rather than relying on a
// propertyKeyNode miss, because the spread's argument identifier would
// otherwise be misread as a key.
func collectObjectKeys(obj *wrapperchecker.Node) []*wrapperchecker.Node {
	if obj == nil || obj.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return nil
	}
	var out []*wrapperchecker.Node
	obj.ForEachChild(func(prop *wrapperchecker.Node) bool {
		switch prop.Kind() {
		case wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			if name := propertyKeyNode(prop); name != nil {
				out = append(out, name)
			}
		}
		return false
	})
	return out
}

// collectVarStatementBindings returns every identifier introduced by
// the declarators in a `const/let/var` statement, descending into
// destructuring patterns.
func collectVarStatementBindings(stmt *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindVariableDeclarationList {
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() == wrapperchecker.KindVariableDeclaration {
					out = append(out, bindingIdentifiers(d)...)
				}
				return false
			})
		}
		return false
	})
	return out
}

// bindingIdentifiers walks a declarator's name slot, descending into
// binding patterns to surface every bound identifier.
func bindingIdentifiers(decl *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindIdentifier:
			out = append(out, n)
		case wrapperchecker.KindObjectBindingPattern, wrapperchecker.KindArrayBindingPattern:
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c)
				return false
			})
		case wrapperchecker.KindBindingElement:
			walk(n.BindingElementName())
		}
	}
	// First name-shaped child is the declaration's binding slot.
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if len(out) > 0 {
			return true
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			walk(c)
		}
		return false
	})
	return out
}

// collectBareDefinePropsKeys returns the prop names declared by a
// `defineProps(...)` call that is itself an expression statement (no
// surrounding assignment). When defineProps is assigned to a variable
// the assignment's own binding is what shows up in scope, so the prop
// names should not be added.
func collectBareDefinePropsKeys(stmt *wrapperchecker.Node) []*wrapperchecker.Node {
	var call *wrapperchecker.Node
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindCallExpression {
			call = c
			return true
		}
		return false
	})
	if call == nil {
		return nil
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier || callee.LiteralText() != "defineProps" {
		return nil
	}
	if first := firstArg(call); first != nil {
		first = unwrap(first)
		switch first.Kind() {
		case wrapperchecker.KindArrayLiteralExpression:
			var out []*wrapperchecker.Node
			first.ForEachChild(func(c *wrapperchecker.Node) bool {
				if c.Kind() == wrapperchecker.KindStringLiteral {
					out = append(out, c)
				}
				return false
			})
			return out
		case wrapperchecker.KindObjectLiteralExpression:
			return collectObjectKeys(first)
		}
	}
	if typeArg := firstTypeArg(call); typeArg != nil {
		return typeArgKeys(typeArg)
	}
	return nil
}

func typeArgKeys(typeArg *wrapperchecker.Node) []*wrapperchecker.Node {
	switch typeArg.Kind() {
	case wrapperchecker.KindTypeLiteral:
		var out []*wrapperchecker.Node
		typeArg.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindPropertySignature {
				if name := propertyKeyNode(c); name != nil {
					out = append(out, name)
				}
			}
			return false
		})
		return out
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
			return nil
		}
		if decl := findTypeDeclByName(typeArg, name); decl != nil {
			var out []*wrapperchecker.Node
			decl.ForEachChild(func(c *wrapperchecker.Node) bool {
				switch c.Kind() {
				case wrapperchecker.KindTypeLiteral:
					out = append(out, typeArgKeys(c)...)
				case wrapperchecker.KindPropertySignature:
					if name := propertyKeyNode(c); name != nil {
						out = append(out, name)
					}
				}
				return false
			})
			return out
		}
	}
	return nil
}

func functionDeclarationName(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var name *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c
			return true
		}
		return false
	})
	return name
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

// inScriptSetup is the cheap marker check (same as no-vue-reserved-keys).
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

func propertyKeyName(prop *wrapperchecker.Node) string {
	if k := propertyKeyNode(prop); k != nil {
		return k.LiteralText()
	}
	return ""
}

func propertyKeyNode(prop *wrapperchecker.Node) *wrapperchecker.Node {
	var key *wrapperchecker.Node
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier, wrapperchecker.KindStringLiteral, wrapperchecker.KindPrivateIdentifier:
			if key == nil {
				key = c
				return true
			}
		}
		return false
	})
	return key
}

// propertyValueAny extracts the value slot of a property, supporting
// `name: expr` (PropertyAssignment), `name() {}` (MethodDeclaration —
// the method itself is the value), and shorthand `name` (the identifier
// doubles as the value).
func propertyValueAny(prop *wrapperchecker.Node) *wrapperchecker.Node {
	switch prop.Kind() {
	case wrapperchecker.KindPropertyAssignment:
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
	case wrapperchecker.KindMethodDeclaration:
		return prop
	case wrapperchecker.KindShorthandPropertyAssignment:
		return prop
	}
	return nil
}

func propertyValueObject(prop *wrapperchecker.Node) *wrapperchecker.Node {
	v := propertyValueAny(prop)
	if v == nil {
		return nil
	}
	v = unwrap(v)
	if v.Kind() == wrapperchecker.KindObjectLiteralExpression {
		return v
	}
	return nil
}

func dataReturnedObject(prop *wrapperchecker.Node) *wrapperchecker.Node {
	if prop.Kind() == wrapperchecker.KindMethodDeclaration {
		return returnedObjectFromBlock(prop)
	}
	value := propertyValueAny(prop)
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

func arrowBody(arrow *wrapperchecker.Node) *wrapperchecker.Node {
	var last *wrapperchecker.Node
	arrow.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

func returnedObjectFromBlock(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var block *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
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
