// Package unboundmethod implements the unbound-method rule: flag a
// reference to an instance method (e.g. `obj.method` without a call,
// without `this: void` annotation) that is likely to lose its `this`
// binding when invoked elsewhere.
//
// Behavioral spec ported from typescript-eslint's unbound-method rule.
// See: https://github.com/typescript-eslint/typescript-eslint/blob/main/packages/eslint-plugin/src/rules/unbound-method.ts
package unboundmethod

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "unbound-method"

// Options configures the rule. `IgnoreStatic` matches the upstream
// option of the same name — when true, static-method references like
// `MyClass.someMethod` aren't flagged.
type Options struct {
	IgnoreStatic bool
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return &rule{} }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: r.visit,
		wrapperchecker.KindObjectBindingPattern:     r.visitObjectPattern,
		wrapperchecker.KindObjectLiteralExpression:  r.visitObjectAssignmentPattern,
	}
}

// visitObjectAssignmentPattern handles destructuring assignment of the
// form `({ method } = instance)` — the object literal is the LHS of an
// assignment, parsed as ObjectLiteralExpression (not a binding pattern)
// because the syntax is ambiguous at parse time. Only visits when the
// object literal is actually a destructuring target.
func (r *rule) visitObjectAssignmentPattern(ctx *engine.Context, n *wrapperchecker.Node) {
	bin := n.Parent()
	if bin == nil {
		return
	}
	// `({ x } = ...)` — the parent is a Parenthesized then a Binary.
	if bin.Kind() == wrapperchecker.KindParenthesizedExpression {
		bin = bin.Parent()
	}
	if bin == nil || bin.Kind() != wrapperchecker.KindBinaryExpression {
		return
	}
	if bin.BinaryOperatorKind() != wrapperchecker.KindEqualsToken {
		return
	}
	left := bin.BinaryLeft()
	for left != nil && left.Kind() == wrapperchecker.KindParenthesizedExpression {
		left = left.FirstChild()
	}
	if left == nil || left != n && !sameNode(left, n) {
		return
	}
	right := bin.BinaryRight()
	if right == nil {
		return
	}
	if right.Kind() == wrapperchecker.KindIdentifier {
		if _, ok := nativelyBoundGlobals[right.LiteralText()]; ok {
			return
		}
	}
	srcT := ctx.TypeOf(right)
	if srcT == nil {
		return
	}
	n.ForEachChild(func(elem *wrapperchecker.Node) bool {
		if elem.Kind() != wrapperchecker.KindShorthandPropertyAssignment {
			return false
		}
		name := elem.FirstChild()
		if name == nil || name.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		propName := name.LiteralText()
		if propName == "" {
			return false
		}
		reportFirstMethodInType(ctx, srcT, propName, name)
		return false
	})
}


func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if isSafeUse(n) {
		return
	}
	if isNativelyBound(ctx, n) {
		return
	}
	sym := propertyMethodSymbol(ctx, n)
	if sym == nil {
		return
	}
	if r.opts.IgnoreStatic && symbolIsStaticMember(sym) {
		return
	}
	checkIfMethodAndReport(ctx, n, sym)
}

// symbolIsStaticMember reports whether the symbol's declaration is a
// static class member (method or getter/setter with the `static`
// keyword). Used by ignoreStatic to skip references to ClassName.member.
func symbolIsStaticMember(sym *wrapperchecker.Symbol) bool {
	if sym == nil {
		return false
	}
	for _, d := range sym.Declarations() {
		switch d.Kind() {
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindMethodSignature,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindPropertyDeclaration,
			wrapperchecker.KindPropertySignature:
			if d.HasStaticModifier() {
				return true
			}
		}
	}
	return false
}

// propertyMethodSymbol resolves the method symbol of `obj.prop`. We
// prefer the property symbol on the receiver's type because
// GetSymbolAtLocation on a PropertyAccessExpression occasionally
// returns nil in tsgo when the access is in argument position.
func propertyMethodSymbol(ctx *engine.Context, access *wrapperchecker.Node) *wrapperchecker.Symbol {
	if sym := ctx.Checker().SymbolOf(access); sym != nil {
		return sym
	}
	recv := access.PropertyAccessReceiver()
	if recv == nil {
		return nil
	}
	recvT := ctx.TypeOf(recv)
	if recvT == nil {
		return nil
	}
	return recvT.PropertySymbol(access.PropertyAccessName())
}

// nativelyBoundGlobals lists JS globals whose static methods are
// `this`-independent — referencing them detached is safe.
var nativelyBoundGlobals = map[string]struct{}{
	"Number":  {},
	"Object":  {},
	"String":  {},
	"RegExp":  {},
	"Symbol":  {},
	"Array":   {},
	"Proxy":   {},
	"Date":    {},
	"Atomics": {},
	"Reflect": {},
	"console": {},
	"Math":    {},
	"JSON":    {},
	"Intl":    {},
	"window":  {},
}

// visitObjectPattern flags `const { method } = instance` patterns
// where `method` is a class method that loses its `this` when extracted.
func (r *rule) visitObjectPattern(ctx *engine.Context, n *wrapperchecker.Node) {
	if isInTypeDeclaration(n) {
		return
	}
	// `const { x } = global` where global is a natively-bound source —
	// destructured methods on Math/JSON/console/etc. are safe.
	if isDestructuringFromNativelyBoundSource(ctx, n) {
		return
	}
	// Resolve the type of the value being destructured. If there's a
	// runtime source (initializer / RHS), use its type; otherwise use
	// the pattern's own type (parameter annotation, contextual type).
	srcT := destructuringType(ctx, n)
	if srcT == nil {
		return
	}
	n.ForEachChild(func(elem *wrapperchecker.Node) bool {
		if elem.Kind() != wrapperchecker.KindBindingElement {
			return false
		}
		var name *wrapperchecker.Node
		elem.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier {
				name = c
				return true
			}
			return false
		})
		if name == nil {
			return false
		}
		propName := name.LiteralText()
		if propName == "" {
			return false
		}
		// Walk union/intersection constituents — a method on any
		// constituent is enough to flag.
		if reportFirstMethodInType(ctx, srcT, propName, name) {
			return false
		}
		return false
	})
}

func reportFirstMethodInType(ctx *engine.Context, t *wrapperchecker.Type, name string, at *wrapperchecker.Node) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if reportFirstMethodInType(ctx, m, name, at) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if reportFirstMethodInType(ctx, m, name, at) {
				return true
			}
		}
		return false
	}
	prop := t.PropertySymbol(name)
	if prop == nil {
		return false
	}
	return checkIfMethodAndReport(ctx, at, prop)
}

// isInTypeDeclaration walks ancestors to detect whether the pattern is
// part of a type declaration (interface/type alias/function type node)
// rather than a runtime binding. Also catches `abstract` methods and
// `declare class`/`declare const` contexts that have no runtime body.
func isInTypeDeclaration(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindInterfaceDeclaration,
			wrapperchecker.KindTypeAliasDeclaration,
			wrapperchecker.KindFunctionType,
			wrapperchecker.KindConstructorType:
			return true
		case wrapperchecker.KindMethodDeclaration:
			if cur.HasAbstractModifier() {
				return true
			}
			return false
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindVariableStatement,
			wrapperchecker.KindFunctionDeclaration:
			if cur.HasDeclareModifier() {
				return true
			}
		case wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction:
			return false
		}
	}
	return false
}

// isDestructuringFromNativelyBoundSource detects `const { x } = Math`
// or similar patterns where the source is a global like Math/JSON/etc.
func isDestructuringFromNativelyBoundSource(ctx *engine.Context, pattern *wrapperchecker.Node) bool {
	p := pattern.Parent()
	if p == nil {
		return false
	}
	var source *wrapperchecker.Node
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		source = p.VariableDeclarationInitializer()
	case wrapperchecker.KindBinaryExpression:
		// `({ x } = Math)` — RHS of an assignment
		if p.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
			// We're the LHS pattern; the RHS is past us — would need
			// dedicated accessor; skip the rare case.
		}
	case wrapperchecker.KindParameter:
		source = p.ParameterInitializer()
	}
	if source == nil {
		return false
	}
	if source.Kind() == wrapperchecker.KindIdentifier {
		name := source.LiteralText()
		if _, ok := nativelyBoundGlobals[name]; ok &&
			!source.SymbolHasUserDeclaration(ctx.Checker()) {
			return true
		}
	}
	srcT := ctx.TypeOf(source)
	if srcT == nil {
		return false
	}
	if _, ok := supportedGlobalTypeSymbolNames[srcT.SymbolName()]; ok &&
		typeOriginatesInDeclarationFile(srcT) {
		return true
	}
	return false
}

func destructuringType(ctx *engine.Context, pattern *wrapperchecker.Node) *wrapperchecker.Type {
	p := pattern.Parent()
	if p == nil {
		return nil
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		if init := p.VariableDeclarationInitializer(); init != nil {
			return ctx.TypeOf(init)
		}
	case wrapperchecker.KindParameter:
		// `function ({ x }: T = init) {}` — prefer annotation; fall
		// back to initializer's type.
		if t := ctx.TypeOf(pattern); t != nil {
			return t
		}
		if init := p.ParameterInitializer(); init != nil {
			return ctx.TypeOf(init)
		}
	}
	// Pattern has no obvious source — try the pattern's own type
	// (works for parameter annotations and type-asserted RHSs).
	return ctx.TypeOf(pattern)
}

// supportedGlobalTypeSymbolNames lists the Type symbol names that
// indicate "this is one of the natively-bound globals" (or its alias).
var supportedGlobalTypeSymbolNames = map[string]struct{}{
	"NumberConstructor": {},
	"ObjectConstructor": {},
	"StringConstructor": {},
	"SymbolConstructor": {},
	"ArrayConstructor":  {},
	"Array":             {},
	"ProxyConstructor":  {},
	"Console":           {},
	"DateConstructor":   {},
	"Atomics":           {},
	"Math":              {},
	"JSON":              {},
}

// isNativelyBound is true when the receiver of `obj.prop` resolves to a
// global like Math/JSON/console whose methods don't rely on `this`.
func isNativelyBound(ctx *engine.Context, access *wrapperchecker.Node) bool {
	recv := access.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	// Direct-name shortcut: `Math.floor`, `JSON.stringify`, etc.
	if recv.Kind() == wrapperchecker.KindIdentifier {
		name := recv.LiteralText()
		if _, ok := nativelyBoundGlobals[name]; ok &&
			!recv.SymbolHasUserDeclaration(ctx.Checker()) {
			return true
		}
	}
	// Type-level fallback: `const foo = Math; foo.floor` — foo's type
	// symbol is `Math`, the property symbol comes from a lib file. The
	// type itself must originate from the default library to avoid
	// matching user-defined classes that happen to share a global
	// type name (`class Console {}`).
	recvT := ctx.TypeOf(recv)
	if recvT == nil {
		return false
	}
	tsym := recvT.SymbolName()
	if _, ok := supportedGlobalTypeSymbolNames[tsym]; !ok {
		return false
	}
	if !typeOriginatesInDeclarationFile(recvT) {
		return false
	}
	propSym := ctx.Checker().SymbolOf(access)
	if propSym == nil {
		return false
	}
	for _, d := range propSym.Declarations() {
		if d.IsInDeclarationFile() {
			return true
		}
	}
	return false
}

func typeOriginatesInDeclarationFile(t *wrapperchecker.Type) bool {
	for _, d := range t.SymbolDeclarations() {
		if !d.IsInDeclarationFile() {
			return false
		}
	}
	return true
}

func checkIfMethodAndReport(ctx *engine.Context, n *wrapperchecker.Node, sym *wrapperchecker.Symbol) bool {
	val := sym.SymbolValueDeclaration()
	if val == nil {
		return false
	}
	if !checkIfMethod(val) {
		return false
	}
	ctx.Report(n, "this method must be called with its receiver — bind, wrap, or use `this: void`")
	return true
}

// checkIfMethod inspects a symbol's value declaration to decide whether
// a detached reference is dangerous.
func checkIfMethod(decl *wrapperchecker.Node) bool {
	switch decl.Kind() {
	case wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindMethodSignature:
		return checkMethodFunc(decl)
	case wrapperchecker.KindPropertyDeclaration:
		// `class C { foo = function() {} }` — function expressions
		// keep their own `this` binding; report.
		init := decl.PropertyDeclarationInitializer()
		return init != nil && init.Kind() == wrapperchecker.KindFunctionExpression
	case wrapperchecker.KindPropertyAssignment:
		// `{ foo: function() {} }` — likewise.
		var lastChild *wrapperchecker.Node
		decl.ForEachChild(func(c *wrapperchecker.Node) bool {
			lastChild = c
			return false
		})
		if lastChild != nil && lastChild.Kind() == wrapperchecker.KindFunctionExpression {
			return checkMethodFunc(lastChild)
		}
		return false
	}
	return false
}

func checkMethodFunc(decl *wrapperchecker.Node) bool {
	params := decl.FunctionParameters()
	if len(params) == 0 {
		return true
	}
	first := params[0]
	name := first.ParameterName()
	if name == nil || name.Kind() != wrapperchecker.KindIdentifier ||
		name.LiteralText() != "this" {
		return true
	}
	annot := first.ParameterTypeAnnotation()
	if annot != nil && annot.IsVoidTypeNode() {
		return false
	}
	return true
}

// isSafeUse mirrors the upstream isSafeUse: contexts where dropping
// `this` cannot cause a problem.
func isSafeUse(n *wrapperchecker.Node) bool {
	parent := n.Parent()
	if parent == nil {
		return false
	}
	switch parent.Kind() {
	case wrapperchecker.KindIfStatement,
		wrapperchecker.KindForStatement,
		wrapperchecker.KindWhileStatement,
		wrapperchecker.KindDoStatement,
		wrapperchecker.KindSwitchStatement,
		wrapperchecker.KindPropertyAccessExpression,
		wrapperchecker.KindElementAccessExpression,
		wrapperchecker.KindPostfixUnaryExpression:
		return true
	case wrapperchecker.KindCallExpression:
		callee := parent.CalleeExpression()
		if callee != nil && sameNode(callee, n) {
			return true
		}
		return false
	case wrapperchecker.KindTaggedTemplateExpression:
		return true
	case wrapperchecker.KindConditionalExpression:
		// `cond ? a : b` — the test position is safe; the branches
		// inherit the parent's safety.
		if cond := parent.ConditionalCondition(); cond != nil && sameNode(cond, n) {
			return true
		}
		return isSafeUse(parent)
	case wrapperchecker.KindPrefixUnaryExpression,
		wrapperchecker.KindTypeOfExpression,
		wrapperchecker.KindDeleteExpression,
		wrapperchecker.KindVoidExpression:
		return true
	case wrapperchecker.KindBinaryExpression:
		switch parent.BinaryOperatorKind() {
		case wrapperchecker.KindEqualsEqualsToken,
			wrapperchecker.KindEqualsEqualsEqualsToken,
			wrapperchecker.KindExclamationEqualsToken,
			wrapperchecker.KindExclamationEqualsEqualsToken:
			return true
		case wrapperchecker.KindEqualsToken:
			if l := parent.BinaryLeft(); l != nil && sameNode(l, n) {
				return true
			}
		case wrapperchecker.KindAmpersandAmpersandToken:
			if l := parent.BinaryLeft(); l != nil && sameNode(l, n) {
				return true
			}
			return isSafeUse(parent)
		case wrapperchecker.KindBarBarToken:
			return isSafeUse(parent)
		}
		return false
	case wrapperchecker.KindNonNullExpression,
		wrapperchecker.KindAsExpression,
		wrapperchecker.KindSatisfiesExpression,
		wrapperchecker.KindParenthesizedExpression,
		wrapperchecker.KindTypeAssertionExpression:
		return isSafeUse(parent)
	}
	return false
}

func sameNode(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	af, asl, asc, ael, aec := a.SourceRange()
	bf, bsl, bsc, bel, bec := b.SourceRange()
	return af == bf && asl == bsl && asc == bsc && ael == bel && aec == bec
}
