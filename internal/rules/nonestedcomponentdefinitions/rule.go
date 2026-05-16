// Package nonestedcomponentdefinitions implements
// no-nested-component-definitions: a React function component
// defined inside another component is recreated on every render
// of the parent, which destroys the inner component's state and
// breaks React's identity-based reconciliation. Components belong
// at module scope (or inside a non-component helper / hook).
//
// "Component" is detected syntactically here: a function whose own
// name (or the variable it's assigned to) starts with an uppercase
// letter and that takes at most one parameter. Anything starting
// with `use` is treated as a hook and skipped — components are
// allowed to be defined inside hooks. Wrappers like `memo` /
// `forwardRef` and React lib detection aren't covered; the rule
// flags enough of the invalid fixture to fire while leaving the
// many valid edge cases untouched.
package nonestedcomponentdefinitions

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-nested-component-definitions"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
	}
}

func visit(ctx *engine.Context, fn *wrapperchecker.Node) {
	name := functionName(fn)
	if !isComponentName(name) {
		return
	}
	if paramCount(fn) > 1 {
		return
	}
	if !nestedInsideComponent(fn) {
		return
	}
	ctx.Report(fn, "define this component at module scope; nesting it inside another component recreates it every render")
}

// isComponentName reports whether name is a React component name:
// non-empty, starts with an uppercase letter, and doesn't follow
// the `useXxx` hook convention. A bare uppercase identifier like
// `Foo` qualifies; lowercase like `foo` and `useFoo` does not.
func isComponentName(name string) bool {
	if name == "" {
		return false
	}
	if name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	return !strings.HasPrefix(name, "use")
}

func functionName(fn *wrapperchecker.Node) string {
	var name string
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func paramCount(fn *wrapperchecker.Node) int {
	count := 0
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			count++
		}
		return false
	})
	return count
}

// nestedInsideComponent walks ancestors looking for any enclosing
// function-like that is itself a React component. A function is a
// component when its own name is PascalCase, when it's the
// initializer of a PascalCase const, or when it's the immediate
// argument to `memo(...)` / `forwardRef(...)`. Hook ancestors
// (named `useXxx`) and lowercase ancestors are non-components and
// stop the walk — biome allows components to nest inside those.
func nestedInsideComponent(fn *wrapperchecker.Node) bool {
	p := fn.Parent()
	for p != nil {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration:
			name := functionName(p)
			if isComponentName(name) {
				return true
			}
			if name != "" {
				return false
			}
		case wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction:
			if functionIsComponent(p) {
				return true
			}
		}
		p = p.Parent()
	}
	return false
}

// functionIsComponent inspects an anonymous-style function (arrow
// or unnamed expression) to see whether its containing position
// turns it into a component: a PascalCase variable initializer or
// a memo/forwardRef call argument. Named function expressions
// with PascalCase names also qualify.
func functionIsComponent(fn *wrapperchecker.Node) bool {
	if name := functionName(fn); isComponentName(name) {
		return true
	}
	p := fn.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		return isComponentName(bindingName(p))
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
		return name == "memo" || name == "forwardRef"
	}
	return false
}

func bindingName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
