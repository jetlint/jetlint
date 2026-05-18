// Package noredundantusestrict implements no-redundant-use-strict
// (biome alias: strict). The classic `"use strict"` directive switches
// a script into strict mode; once strict mode is already active for
// the surrounding scope it is noise to repeat the directive.
//
// A `"use strict"` directive is redundant when any of these hold:
//   - the source file is an ES module (every module is always strict),
//   - a class wraps the directive (class bodies are always strict),
//   - an enclosing function body or script prologue already opens with
//     `"use strict"`,
//   - the directive is not the first `"use strict"` of its own
//     prologue (the prior copy already activated strict mode).
//
// Other prologue strings such as `'use client'` or `'use server'` are
// not flagged — only `"use strict"` itself.
package noredundantusestrict

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-redundant-use-strict"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindExpressionStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isUseStrictDirective(n) {
		return
	}
	container := n.Parent()
	if container == nil || !isPrologueContainer(container) {
		return
	}
	priorUseStrict, ok := prologuePosition(container, n)
	if !ok {
		return
	}
	if containerIsStrict(container) || priorUseStrict > 0 {
		ctx.Report(n, `Redundant "use strict" directive.`)
	}
}

// isUseStrictDirective reports whether n is an ExpressionStatement
// whose expression is the literal `"use strict"` (or its single-quoted
// equivalent).
func isUseStrictDirective(n *wrapperchecker.Node) bool {
	if n.Kind() != wrapperchecker.KindExpressionStatement {
		return false
	}
	expr := n.ExpressionStatementExpression()
	if expr == nil || expr.Kind() != wrapperchecker.KindStringLiteral {
		return false
	}
	txt := strings.TrimSpace(expr.SourceText())
	return txt == `"use strict"` || txt == `'use strict'`
}

// isPrologueContainer reports whether container is a place where
// directive prologues live — a source file, a namespace body, or a
// function/method/arrow body block.
func isPrologueContainer(container *wrapperchecker.Node) bool {
	switch container.Kind() {
	case wrapperchecker.KindSourceFile, wrapperchecker.KindModuleBlock:
		return true
	case wrapperchecker.KindBlock:
		grand := container.Parent()
		return grand != nil && isFunctionLike(grand.Kind())
	}
	return false
}

// prologuePosition counts how many `"use strict"` directives precede
// target within container's directive prologue. Returns (count, true)
// when target is itself part of the prologue, or (_, false) when a
// non-directive statement appears before target.
func prologuePosition(container, target *wrapperchecker.Node) (int, bool) {
	var prior int
	var hitNonDirective bool
	var found bool
	container.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Inner() == target.Inner() {
			found = true
			return true
		}
		if hitNonDirective {
			return false
		}
		if !isStringDirective(c) {
			hitNonDirective = true
			return false
		}
		if isUseStrictDirective(c) {
			prior++
		}
		return false
	})
	if !found || hitNonDirective {
		return 0, false
	}
	return prior, true
}

// isStringDirective reports whether c is an ExpressionStatement of a
// bare string literal — the shape directive prologues take.
func isStringDirective(c *wrapperchecker.Node) bool {
	if c.Kind() != wrapperchecker.KindExpressionStatement {
		return false
	}
	expr := c.ExpressionStatementExpression()
	if expr == nil {
		return false
	}
	return expr.Kind() == wrapperchecker.KindStringLiteral ||
		expr.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral
}

// containerIsStrict reports whether container's scope is strict
// independent of any `"use strict"` directive within container itself.
// Triggers: container is a module source file, a TS namespace body, a
// function/method body wrapped in a class, or a function body whose
// outer function/script already has `"use strict"`.
func containerIsStrict(container *wrapperchecker.Node) bool {
	switch container.Kind() {
	case wrapperchecker.KindSourceFile:
		return sourceFileIsModule(container)
	case wrapperchecker.KindModuleBlock:
		return true
	}
	for cur := container.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
			return true
		case wrapperchecker.KindModuleBlock:
			return true
		case wrapperchecker.KindSourceFile:
			if sourceFileIsModule(cur) {
				return true
			}
			return hasUseStrictPrologue(cur)
		}
		if cur.Kind() == wrapperchecker.KindBlock {
			grand := cur.Parent()
			if grand != nil && isFunctionLike(grand.Kind()) {
				if hasUseStrictPrologue(cur) {
					return true
				}
			}
		}
	}
	return false
}

// sourceFileIsModule reports whether sf has any top-level import or
// export — the syntactic signal that turns a file into an ES module.
func sourceFileIsModule(sf *wrapperchecker.Node) bool {
	isModule := false
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindImportDeclaration,
			wrapperchecker.KindExportDeclaration,
			wrapperchecker.KindExportAssignment:
			isModule = true
			return true
		}
		if c.HasExportModifier() {
			isModule = true
			return true
		}
		return false
	})
	return isModule
}

// hasUseStrictPrologue reports whether the first statement of n is a
// `"use strict"` directive. Used to detect strict inheritance from an
// enclosing function or script.
func hasUseStrictPrologue(n *wrapperchecker.Node) bool {
	found := false
	first := true
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !first {
			return true
		}
		first = false
		if isUseStrictDirective(c) {
			found = true
		}
		return true
	})
	return found
}

func isFunctionLike(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindConstructor:
		return true
	}
	return false
}
