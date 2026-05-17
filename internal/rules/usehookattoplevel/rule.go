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
// Test-framework helpers that happen to follow the same naming
// (`jest.useFakeTimers`, `vi.useRealTimers`) are excluded.
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
		if isTestFrameworkUseCall(callee) {
			return false
		}
		name = callee.PropertyAccessName()
	default:
		return false
	}
	return isHookName(name)
}

// isTestFrameworkUseCall reports whether the property access has the
// shape `jest.useX` / `vi.useX` / `vitest.useX` / `sinon.useX`.
// These are test-runner clock controls, not React hooks.
func isTestFrameworkUseCall(access *wrapperchecker.Node) bool {
	var head *wrapperchecker.Node
	access.ForEachChild(func(c *wrapperchecker.Node) bool {
		if head == nil {
			head = c
			return true
		}
		return false
	})
	if head == nil || head.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	switch head.LiteralText() {
	case "jest", "vi", "vitest", "sinon":
		return true
	}
	return false
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
// a React component or custom hook by naming convention. Methods and
// accessors qualify when their own name follows the hook/component
// convention (`useFoo`, `MyMethod`) — biome treats `class X { useY()
// {...} }` as a hook because the name carries the same signal as a
// standalone function. Function expressions and arrows take their
// name from the variable, property, or parameter they were assigned
// to (or their own inline name), or from a `forwardRef`/`memo`
// wrapper.
func isComponentOrHook(host *wrapperchecker.Node, kind hostKind) bool {
	if host == nil {
		return false
	}
	switch kind {
	case hostKindMethod, hostKindAccessor:
		name := methodOrAccessorName(host)
		return isComponentName(name) || isHookName(name)
	case hostKindFunctionDecl:
		name := functionDeclName(host)
		return isComponentName(name) || isHookName(name)
	case hostKindFunctionExpr, hostKindArrow:
		name := implicitName(host)
		if name == "" && kind == hostKindFunctionExpr {
			name = functionDeclName(host)
		}
		if isComponentName(name) || isHookName(name) {
			return true
		}
		// Wrapped as `memo(Component)` / `forwardRef(Component)` —
		// the wrapper conveys "this is a component."
		if wrappedAsComponent(host) {
			return true
		}
		// Passed as the callback to a known test/render helper
		// (`test`, `it`, `describe`, `beforeEach`, `renderHook`, …)
		// — these run the callback as if it were a component setup,
		// so hooks inside are permitted.
		if insideTestWrapperCallback(host) {
			return true
		}
		// Concise arrow body whose entire body IS a hook call —
		// `(x) => useFoo(x)` — is treated as a hook wrapper by
		// biome. The pattern names a passthrough that the caller
		// will use exactly where a hook would go.
		if kind == hostKindArrow && conciseBodyIsHookCall(host) {
			return true
		}
		return false
	}
	return false
}

// insideTestWrapperCallback returns true when fn is the callback
// argument of a recognized test / renderer helper call.
func insideTestWrapperCallback(fn *wrapperchecker.Node) bool {
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
	var name string
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		name = callee.LiteralText()
	case wrapperchecker.KindPropertyAccessExpression:
		name = callee.PropertyAccessName()
	}
	switch name {
	case "test", "it", "describe", "beforeEach", "afterEach",
		"beforeAll", "afterAll", "renderHook", "render":
		return true
	}
	return false
}

// conciseBodyIsHookCall returns true when arrow is a concise-body
// arrow whose body is exactly a single hook call expression
// (`(x) => useFoo(x)`), the implicit-return wrapper idiom biome
// allows even when the surrounding name doesn't follow the
// component/hook convention.
func conciseBodyIsHookCall(arrow *wrapperchecker.Node) bool {
	body := functionBodyExpr(arrow)
	if body == nil {
		return false
	}
	body = unwrapParens(body)
	if body.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := body.CalleeExpression()
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return isHookName(callee.LiteralText())
	case wrapperchecker.KindPropertyAccessExpression:
		return isHookName(callee.PropertyAccessName())
	}
	return false
}

// functionBodyExpr returns an arrow's concise body expression, or nil
// for block-bodied arrows.
func functionBodyExpr(arrow *wrapperchecker.Node) *wrapperchecker.Node {
	var last *wrapperchecker.Node
	arrow.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	if last == nil || last.Kind() == wrapperchecker.KindBlock {
		return nil
	}
	return last
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

// methodOrAccessorName returns the property-name slot of a method,
// accessor, or class property declaration.
func methodOrAccessorName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier, wrapperchecker.KindStringLiteral, wrapperchecker.KindPrivateIdentifier:
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
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
		case wrapperchecker.KindPropertyDeclaration:
			// Class field: `useHook = () => {...}` — the field's
			// name is the hook/component name.
			return methodOrAccessorName(p)
		case wrapperchecker.KindExportAssignment:
			return "Default"
		}
		return ""
	}
	return ""
}

// wrappedAsComponent returns true when fn appears as the argument of
// a `memo(...)` or `forwardRef(...)` call (possibly nested through
// either wrapper). Reaching such a wrapper at any depth is enough;
// the wrapper itself is the "this is a component" pragma, and the
// surrounding code may not even bind the result to a variable.
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
					return true
				}
			}
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
// loops, try, ternaries, the right operand of short-circuit
// operators, and try blocks. finally blocks are NOT disqualifying —
// code in finally always runs. The LEFT operand of `&&`, `||`, or
// `??` always runs unconditionally, so a hook there is allowed
// (`const v = useFoo() ?? fallback`).
func onDisqualifyingPath(call, host *wrapperchecker.Node) bool {
	prev := call
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
			if isShortCircuit(cur) && isRightOperand(cur, prev) {
				return true
			}
		case wrapperchecker.KindTryStatement:
			if !inFinallyClause(cur, call) {
				return true
			}
		case wrapperchecker.KindCatchClause:
			return true
		}
		prev = cur
		cur = cur.Parent()
	}
	return false
}

// isRightOperand reports whether child sits on the right side of a
// binary operator. tsgo's BinaryExpression children come in
// [left, operatorToken, right] order; the right side is the last.
func isRightOperand(bin, child *wrapperchecker.Node) bool {
	var last *wrapperchecker.Node
	bin.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	if last == nil {
		return false
	}
	return last.Pos() == child.Pos() && last.End() == child.End()
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
