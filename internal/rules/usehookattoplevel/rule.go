// Package usehookattoplevel implements use-hook-at-top-level: React
// requires hook calls to happen on every render in the same order, so
// they may only appear at the top level of a component or custom hook.
// The rule flags hook calls (callees starting with `use` followed by
// an uppercase letter, or property-accesses with the same shape) that
// are conditionally executed (inside `if`, `?:`, `&&`, loops, `try`,
// etc.) or that sit in a function that React would not treat as a
// component or hook.
package usehookattoplevel

import (
	"unicode"
	"unicode/utf8"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-hook-at-top-level"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCall,
	}
}

func visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	if !isHookCall(call) {
		return
	}
	host, hostKind := enclosingFunctionLike(call)
	if hostKind == hostKindModule {
		ctx.Report(call, "This hook is being called from a non-React function. Hooks may only be called from components or hooks.")
		return
	}
	if !isComponentOrHook(host, hostKind) {
		ctx.Report(call, "This hook is being called from a non-React function. Hooks may only be called from components or hooks.")
		return
	}
	if onDisqualifyingPath(call, host) {
		ctx.Report(call, "This hook is called conditionally. Hooks may only be called at the top level of a component or hook.")
		return
	}
}

// isHookCall returns true when call's callee names a React hook by
// convention — an identifier (or property-access tail) that starts
// with `use` followed by an uppercase letter or is exactly `use`.
func isHookCall(call *wrapperchecker.Node) bool {
	callee := call.CalleeExpression()
	if callee == nil {
		return false
	}
	var name string
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		name = callee.LiteralText()
	case wrapperchecker.KindPropertyAccessExpression:
		name = callee.PropertyAccessName()
	default:
		return false
	}
	return isHookName(name)
}

func isHookName(name string) bool {
	if len(name) < 3 || name[:3] != "use" {
		return false
	}
	if len(name) == 3 {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[3:])
	return unicode.IsUpper(r)
}

type hostKind int

const (
	hostKindModule hostKind = iota
	hostKindFunctionDecl
	hostKindFunctionExpr
	hostKindArrow
	hostKindMethod
	hostKindAccessor
)

// enclosingFunctionLike walks up from n and returns the nearest
// function/method/arrow node, plus a kind tag distinguishing the
// flavor. Reports hostKindModule when no such ancestor exists.
func enclosingFunctionLike(n *wrapperchecker.Node) (*wrapperchecker.Node, hostKind) {
	cur := n.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration:
			return cur, hostKindFunctionDecl
		case wrapperchecker.KindFunctionExpression:
			return cur, hostKindFunctionExpr
		case wrapperchecker.KindArrowFunction:
			return cur, hostKindArrow
		case wrapperchecker.KindMethodDeclaration:
			return cur, hostKindMethod
		case wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor:
			return cur, hostKindAccessor
		case wrapperchecker.KindSourceFile:
			return nil, hostKindModule
		}
		cur = cur.Parent()
	}
	return nil, hostKindModule
}

// isComponentOrHook reports whether the enclosing function looks like
// a React component or custom hook by naming convention. Methods,
// accessors, and getters never qualify regardless of name. Function
// expressions and arrows take their name from the variable, property,
// or parameter they were assigned to, or from a `forwardRef`/`memo`
// wrapper.
func isComponentOrHook(host *wrapperchecker.Node, kind hostKind) bool {
	if host == nil {
		return false
	}
	switch kind {
	case hostKindMethod, hostKindAccessor:
		return false
	case hostKindFunctionDecl:
		name := functionDeclName(host)
		return isComponentName(name) || isHookName(name)
	case hostKindFunctionExpr, hostKindArrow:
		name := implicitName(host)
		if isComponentName(name) || isHookName(name) {
			return true
		}
		// Wrapped as `memo(Component)` / `forwardRef(Component)` /
		// `memo(forwardRef(Component))` — the wrapper conveys
		// "this is a component," matching biome's pragma.
		return wrappedAsComponent(host)
	}
	return false
}

// implicitName extracts the name the function expression / arrow is
// being assigned to, propagating through forwardRef/memo wrappers
// so `const X = memo(forwardRef(...))` is named X.
func implicitName(fn *wrapperchecker.Node) string {
	p := fn.Parent()
	for p != nil {
		switch p.Kind() {
		case wrapperchecker.KindParenthesizedExpression:
			p = p.Parent()
			continue
		case wrapperchecker.KindCallExpression:
			callee := p.CalleeExpression()
			if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier {
				switch callee.LiteralText() {
				case "memo", "forwardRef":
					p = p.Parent()
					continue
				}
			}
			return ""
		case wrapperchecker.KindVariableDeclaration:
			return variableDeclName(p)
		case wrapperchecker.KindPropertyAssignment:
			return propertyKeyName(p)
		case wrapperchecker.KindExportAssignment:
			return "Default"
		}
		return ""
	}
	return ""
}

func wrappedAsComponent(fn *wrapperchecker.Node) bool {
	p := fn.Parent()
	for p != nil {
		switch p.Kind() {
		case wrapperchecker.KindParenthesizedExpression:
			p = p.Parent()
			continue
		case wrapperchecker.KindCallExpression:
			callee := p.CalleeExpression()
			if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier {
				switch callee.LiteralText() {
				case "memo", "forwardRef":
					p = p.Parent()
					continue
				}
			}
			return false
		}
		return false
	}
	return false
}

func functionDeclName(fn *wrapperchecker.Node) string {
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

func variableDeclName(decl *wrapperchecker.Node) string {
	var name string
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func propertyKeyName(prop *wrapperchecker.Node) string {
	var name string
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier, wrapperchecker.KindStringLiteral:
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func isComponentName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// onDisqualifyingPath walks from call up to host checking whether any
// intermediate node represents conditional or repeated execution: if,
// loops, try, ternaries, short-circuit operators, .map()-style
// callbacks, and JSX event handlers (function expressions inside JSX).
// finally blocks are NOT disqualifying — code in finally always runs.
func onDisqualifyingPath(call, host *wrapperchecker.Node) bool {
	cur := call.Parent()
	for cur != nil && cur != host {
		switch cur.Kind() {
		case wrapperchecker.KindIfStatement,
			wrapperchecker.KindForStatement,
			wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindConditionalExpression,
			wrapperchecker.KindSwitchStatement:
			return true
		case wrapperchecker.KindBinaryExpression:
			if isShortCircuit(cur) {
				return true
			}
		case wrapperchecker.KindTryStatement:
			if !inFinallyClause(cur, call) {
				return true
			}
		case wrapperchecker.KindCatchClause:
			return true
		}
		cur = cur.Parent()
	}
	return false
}

func isShortCircuit(bin *wrapperchecker.Node) bool {
	switch bin.BinaryOperatorKind() {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken,
		wrapperchecker.KindQuestionQuestionToken:
		return true
	}
	return false
}

// inFinallyClause reports whether call is positioned inside the
// finally clause of tryStmt rather than the try block (or a catch).
// The finally block is the only one that always executes, so hooks
// there preserve the call-order invariant.
//
// Nodes are compared by source position rather than pointer: the
// wrapper layer allocates a fresh *Node every ForEachChild call, so
// two wrapper nodes referring to the same source position are
// distinct pointers.
func inFinallyClause(tryStmt *wrapperchecker.Node, call *wrapperchecker.Node) bool {
	var finallyPos, finallyEnd int
	hasFinally := false
	seenTry := false
	tryStmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			if !seenTry {
				seenTry = true
			} else {
				finallyPos, finallyEnd = c.Pos(), c.End()
				hasFinally = true
				return true
			}
		}
		return false
	})
	if !hasFinally {
		return false
	}
	tryPos, tryEnd := tryStmt.Pos(), tryStmt.End()
	cur := call.Parent()
	for cur != nil && !(cur.Pos() == tryPos && cur.End() == tryEnd) {
		if cur.Pos() == finallyPos && cur.End() == finallyEnd {
			return true
		}
		cur = cur.Parent()
	}
	return false
}
