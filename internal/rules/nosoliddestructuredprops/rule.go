// Package nosoliddestructuredprops implements
// no-solid-destructured-props: Solid's props object is a reactive
// proxy whose getters track reads; destructuring at the parameter
// flushes those reads at component setup time and severs the
// dependency. The fix is `props.x` or `splitProps`. The rule fires
// when a PascalCase component (arrow / function expression / named
// function) takes exactly one parameter and that parameter is an
// object binding pattern.
package nosoliddestructuredprops

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-solid-destructured-props"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindFunctionDeclaration: visit,
	}
}

func visit(ctx *engine.Context, fn *wrapperchecker.Node) {
	if !isComponent(fn) {
		return
	}
	params := collectParameters(fn)
	if len(params) != 1 {
		return
	}
	p := params[0]
	if !parameterIsObjectPattern(p) {
		return
	}
	ctx.Report(p, "destructuring a Solid component's props breaks reactivity; read with `props.x` or use `splitProps`")
}

func isComponent(fn *wrapperchecker.Node) bool {
	if name := selfName(fn); name != "" && startsUpper(name) {
		return true
	}
	if name := boundName(fn); name != "" && startsUpper(name) {
		return true
	}
	return false
}

func startsUpper(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return c >= 'A' && c <= 'Z'
}

func selfName(fn *wrapperchecker.Node) string {
	if fn.Kind() != wrapperchecker.KindFunctionDeclaration &&
		fn.Kind() != wrapperchecker.KindFunctionExpression {
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

func parameterIsObjectPattern(p *wrapperchecker.Node) bool {
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
