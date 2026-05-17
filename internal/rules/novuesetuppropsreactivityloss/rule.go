// Package novuesetuppropsreactivityloss implements
// no-vue-setup-props-reactivity-loss: Vue's setup function receives
// a reactive props proxy. Destructuring it in the parameter list
// reads each value at setup-time once, severing the proxy and
// losing reactive updates. The rule flags any setup method/arrow
// inside a Vue component descriptor (export default, exported
// const, defineComponent/createApp/new Vue) whose first parameter
// is an object binding pattern.
package novuesetuppropsreactivityloss

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-vue-setup-props-reactivity-loss"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindObjectLiteralExpression: visit,
	}
}

func visit(ctx *engine.Context, obj *wrapperchecker.Node) {
	if !isVueComponentDescriptor(obj) {
		return
	}
	setup := findSetupProperty(obj)
	if setup == nil {
		return
	}
	fn := setupFunction(setup)
	if fn == nil {
		return
	}
	params := collectParameters(fn)
	if len(params) == 0 {
		return
	}
	first := params[0]
	if !parameterIsDestructured(first) {
		return
	}
	ctx.Report(first, "destructuring the setup props parameter loses Vue reactivity; access them via `props.x` or use `toRefs`")
}

func findSetupProperty(obj *wrapperchecker.Node) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	obj.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindMethodDeclaration, wrapperchecker.KindPropertyAssignment:
			if propertyKeyName(c) == "setup" {
				found = c
				return true
			}
		}
		return false
	})
	return found
}

func setupFunction(prop *wrapperchecker.Node) *wrapperchecker.Node {
	if prop.Kind() == wrapperchecker.KindMethodDeclaration {
		return prop
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
	if value == nil {
		return nil
	}
	for value.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		value.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner == nil {
			break
		}
		value = inner
	}
	switch value.Kind() {
	case wrapperchecker.KindFunctionExpression, wrapperchecker.KindArrowFunction:
		return value
	}
	return nil
}

func collectParameters(fn *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			out = append(out, c)
		}
		return false
	})
	return out
}

func parameterIsDestructured(p *wrapperchecker.Node) bool {
	var hit bool
	p.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindObjectBindingPattern {
			hit = true
			return true
		}
		return false
	})
	return hit
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

// isVueComponentDescriptor recognizes Vue component shapes:
//   - `export default { ... }`
//   - `export const PascalCase = { ... }`
//   - `defineComponent({...})`, `createApp({...})`, `new Vue({...})`
//
// Bare `const obj = { setup({...}) {} }` is not a component, so
// destructuring its setup param is harmless and shouldn't fire.
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
	case wrapperchecker.KindVariableDeclaration:
		return variableDeclarationIsExportedPascalCase(p)
	}
	return false
}

// variableDeclarationIsExportedPascalCase reports whether the
// declaration is the initializer of an exported PascalCase const,
// the convention Vue users follow when naming a component literal.
func variableDeclarationIsExportedPascalCase(decl *wrapperchecker.Node) bool {
	var name string
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	if name == "" {
		return false
	}
	c := name[0]
	if c < 'A' || c > 'Z' {
		return false
	}
	for p := decl.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindVariableStatement:
			var hasExport bool
			p.ForEachChild(func(child *wrapperchecker.Node) bool {
				if child.Kind() == wrapperchecker.KindExportKeyword {
					hasExport = true
					return true
				}
				return false
			})
			return hasExport
		case wrapperchecker.KindSourceFile:
			return false
		}
	}
	return false
}
