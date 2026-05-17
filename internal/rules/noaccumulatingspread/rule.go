// Package noaccumulatingspread implements no-accumulating-spread:
// flag `[...acc, x]` / `{...acc, x}` patterns inside the callback
// of `.reduce(...)` or `.reduceRight(...)`. Each iteration creates
// a new array/object that copies every element of the accumulator,
// turning an O(n) reduction into O(n²).
//
// The rule matches biome's behavior: only flag spreads whose
// spread target is the first parameter of the immediately enclosing
// reduce-style callback. Spreading anything else is fine.
package noaccumulatingspread

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-accumulating-spread"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSpreadElement:    visitSpread,
		wrapperchecker.KindSpreadAssignment: visitSpread,
		wrapperchecker.KindCallExpression:   visitCall,
	}
}

// visitCall handles `Object.assign(<fresh>, acc, ...)` — the same
// quadratic-copy antipattern dressed up as a method call. Biome
// flags it when any argument after the first is the accumulator
// identifier.
func visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "assign" {
		return
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier || recv.LiteralText() != "Object" {
		return
	}
	fn := enclosingReduceCallback(call)
	if fn == nil {
		return
	}
	accName := firstParamName(fn)
	if accName == "" {
		return
	}
	args := call.CallArguments()
	for i, a := range args {
		if i == 0 {
			continue
		}
		if a.Kind() == wrapperchecker.KindIdentifier && a.LiteralText() == accName {
			ctx.Report(call, "Object.assign on the accumulator inside reduce is the same quadratic-copy antipattern as spreading it")
			return
		}
	}
}

func visitSpread(ctx *engine.Context, spread *wrapperchecker.Node) {
	// The spread target is the single expression child.
	var target *wrapperchecker.Node
	spread.ForEachChild(func(c *wrapperchecker.Node) bool {
		target = c
		return true
	})
	if target == nil || target.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	name := target.LiteralText()
	fn := enclosingReduceCallback(spread)
	if fn == nil {
		return
	}
	if firstParamName(fn) != name {
		return
	}
	ctx.Report(spread, "spreading the accumulator inside reduce turns the loop quadratic — push or assign instead")
}

// enclosingReduceCallback walks parents until it finds a function /
// arrow whose enclosing CallExpression's callee is
// `<x>.reduce(...)` or `<x>.reduceRight(...)`. Returns the function
// node so the caller can inspect its first parameter.
func enclosingReduceCallback(n *wrapperchecker.Node) *wrapperchecker.Node {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
			call := p.Parent()
			if call == nil || call.Kind() != wrapperchecker.KindCallExpression {
				return nil
			}
			callee := call.CalleeExpression()
			if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
				return nil
			}
			name := callee.PropertyAccessName()
			if name == "reduce" || name == "reduceRight" {
				return p
			}
			return nil
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			// Reduce callbacks aren't named functions; treat any
			// other function boundary as exiting the scope.
			return nil
		}
	}
	return nil
}

// firstParamName returns the identifier name of the function's
// first parameter, or "" if it's a destructure / has no params.
func firstParamName(fn *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			first = c
			return true
		}
		return false
	})
	if first == nil {
		return ""
	}
	var name string
	first.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
