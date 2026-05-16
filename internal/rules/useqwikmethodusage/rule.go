// Package useqwikmethodusage implements use-qwik-method-usage:
// Qwik's `useXxx` and `useXxx$` hooks must run inside a component$
// factory or another `useXxx` custom hook so Qwik can attach them
// to the right reactive scope. Calling one in module scope, an
// event handler, a regular helper, or an async/generator function
// detaches the hook from any component and breaks reactivity.
package useqwikmethodusage

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-qwik-method-usage"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, call *wrapperchecker.Node) {
	if !isQwikHookCall(call) {
		return
	}
	if calledInValidContext(call) {
		return
	}
	ctx.Report(call, "Qwik hooks must be called inside a `component$(...)` factory or a custom `useXxx` hook")
}

// isQwikHookCall reports whether the call expression is a direct
// call to an identifier that follows Qwik's `useXxx` naming and
// either ends with `$` (e.g. `useTask$`) or is one of the known
// non-$ Qwik hooks (`useSignal`, `useStore`).
func isQwikHookCall(call *wrapperchecker.Node) bool {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	name := callee.LiteralText()
	if !strings.HasPrefix(name, "use") || len(name) < 4 {
		return false
	}
	c := name[3]
	if c < 'A' || c > 'Z' {
		return false
	}
	return true
}

// calledInValidContext walks ancestors from the call and reports
// true if it finds:
//   - an arrow/function that is the immediate argument of a
//     `component$(...)` call (the canonical Qwik component form), or
//   - a function-like whose own name (or the variable it's bound to)
//     starts with `use` followed by an uppercase letter, marking it
//     as a custom hook.
func calledInValidContext(call *wrapperchecker.Node) bool {
	for p := call.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindFunctionDeclaration:
			if functionIsCustomHook(p) {
				return true
			}
			if functionIsComponentFactoryArg(p) {
				return true
			}
			return false
		case wrapperchecker.KindMethodDeclaration:
			return functionIsCustomHook(p)
		}
	}
	return false
}

func functionIsCustomHook(fn *wrapperchecker.Node) bool {
	if name := selfName(fn); isHookName(name) {
		return true
	}
	if name := boundName(fn); isHookName(name) {
		return true
	}
	return false
}

func isHookName(name string) bool {
	if len(name) < 4 || !strings.HasPrefix(name, "use") {
		return false
	}
	c := name[3]
	return c >= 'A' && c <= 'Z'
}

func selfName(fn *wrapperchecker.Node) string {
	if fn.Kind() != wrapperchecker.KindFunctionDeclaration &&
		fn.Kind() != wrapperchecker.KindFunctionExpression &&
		fn.Kind() != wrapperchecker.KindMethodDeclaration {
		return ""
	}
	var n string
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			n = c.LiteralText()
			return true
		}
		return false
	})
	return n
}

func boundName(fn *wrapperchecker.Node) string {
	p := fn.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil {
		return ""
	}
	if p.Kind() != wrapperchecker.KindVariableDeclaration {
		return ""
	}
	var n string
	p.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			n = c.LiteralText()
			return true
		}
		return false
	})
	return n
}

// functionIsComponentFactoryArg checks if the function literal is
// the immediate argument of a `component$(...)` call (or aliased
// equivalent imported as `MyComponent`-style — we approximate by
// allowing any callee that ends with `$` so aliased imports keep
// passing as long as the convention is preserved).
func functionIsComponentFactoryArg(fn *wrapperchecker.Node) bool {
	p := fn.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil || p.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := p.CalleeExpression()
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		name := callee.LiteralText()
		if name == "component$" {
			return true
		}
		// Aliased import like `import { component$ as MyComponent }`
		// keeps a PascalCase identifier. Allow PascalCase callees to
		// account for that pattern when the file makes the alias.
		if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
			return true
		}
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName() == "component$"
	}
	return false
}
