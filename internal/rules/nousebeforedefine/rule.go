// Package nousebeforedefine implements the no-use-before-define rule:
// referencing a `let`/`const`/`class`/`enum` (and, depending on
// options, `var`/`function`) before its declaration is in the same
// source file is reported as a temporal-dead-zone style hazard. We
// rely on TS-go's symbol resolver to map each identifier reference
// to its declaration, then compare source positions.
package nousebeforedefine

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-use-before-define"

// Options configures the rule.
type Options struct {
	// Functions, when true, flags function declarations used before
	// their declaration position. ESLint's default is true; the
	// "nofunc" shorthand option sets this to false.
	Functions bool
	// Classes, when true, flags class declarations used before
	// their declaration position. ESLint's default is true.
	Classes bool
	// Variables, when true, flags variable declarations used before
	// their declaration position. ESLint's default is true.
	Variables bool
	// AllowNamedExports allows `export { foo }; const foo = 1;`
	// (named exports referencing later-declared values).
	AllowNamedExports bool
	// IgnoreTypeReferences skips identifiers used in type positions
	// (TS-specific). ESLint's default is true.
	IgnoreTypeReferences bool
}

// DefaultOptions returns ESLint's defaults.
func DefaultOptions() Options {
	return Options{
		Functions:            true,
		Classes:              true,
		Variables:            true,
		IgnoreTypeReferences: true,
	}
}

// New constructs the rule with default options.
func New() engine.Rule { return &rule{opts: DefaultOptions()} }

// NewWithOptions constructs the rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isFreeReferenceContext(n) {
		return
	}
	sym := ctx.Checker().SymbolOf(n)
	if sym == nil {
		return
	}
	decls := sym.Declarations()
	if len(decls) == 0 {
		return
	}
	// Only compare positions when the reference and declaration
	// live in the same source file — cross-file (lib globals,
	// imports) positions are not commensurable.
	refFile, _, _, _, _ := n.SourceRange()
	var first *wrapperchecker.Node
	for _, d := range decls {
		df, _, _, _, _ := d.SourceRange()
		if df != refFile {
			continue
		}
		if first == nil || d.Pos() < first.Pos() {
			first = d
		}
	}
	if first == nil {
		return
	}
	if n.Pos() >= first.Pos() {
		// The reference sits at or after the declaration name. A
		// self-reference inside the declaration's *initializer* /
		// default expression — `var a = a;`, `function f(a = a) {}`,
		// `var {a = a} = ...;`, `for (var a in a) {}` — reads the
		// binding before the initializer completes (TDZ for
		// let/const; undefined for var). Flag those. Self-references
		// nested in a deferred body (function / method / arrow
		// body, class block) are still allowed because they run
		// after the declaration completes.
		if isSelfInitReference(first, n) ||
			isClassHeritageSelfReference(first, n) ||
			isForInOfIterableSelfReference(first, n) {
			ctx.Report(n, "'"+n.SourceText()+"' was used before it was defined.")
		}
		return
	}
	// Apply hoisting rules: function declarations and `var`s are
	// hoisted; only flag if the corresponding option says so.
	switch declarationKind(first) {
	case declFunction:
		if !r.opts.Functions {
			return
		}
	case declClass:
		if !r.opts.Classes {
			return
		}
	case declVar:
		if !r.opts.Variables {
			return
		}
	case declParameter, declImport, declOther:
		// Parameters and imports are reachable before any
		// statement, so the rule never flags them.
		return
	}
	ctx.Report(n, "'"+n.SourceText()+"' was used before it was defined.")
}

// isSelfInitReference reports whether `ref` reads the binding being
// declared by `decl` — appearing inside the same VariableDeclaration
// / Parameter / BindingElement's initializer or default expression,
// not nested in a deferred function/method/class body. When `decl`
// is a BindingElement, the "self-init" range covers the element's
// own default expression plus the enclosing VariableDeclaration /
// Parameter's initializer (which runs before destructuring) — but
// not a sibling binding element's default that appears textually
// after this binding (destructuring runs left-to-right).
func isSelfInitReference(decl, ref *wrapperchecker.Node) bool {
	if !inSelfInitRange(decl, ref) {
		return false
	}
	return !crossesDeferredBoundary(ref, decl)
}

// crossesDeferredBoundary walks parents of `ref` toward `decl` and
// reports whether the walk passes through a deferred body — function,
// method, arrow, or a class body whose initialization is not part of
// the surrounding eager-evaluation chain. A class's heritage clause
// and a method/property's computed name are still considered eager.
func crossesDeferredBoundary(ref, decl *wrapperchecker.Node) bool {
	prev := ref
	for cur := ref.Parent(); cur != nil && !cur.Same(decl); cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return true
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindPropertyDeclaration:
			// Computed names of methods / fields are evaluated at
			// class-create time; their bodies / initializers are not.
			if prev.Kind() != wrapperchecker.KindComputedPropertyName {
				return true
			}
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			// A class's heritage clause runs eagerly when the class
			// expression is evaluated; its body / methods do not.
			if prev.Kind() != wrapperchecker.KindHeritageClause {
				return true
			}
		}
		prev = cur
	}
	return false
}

// isClassHeritageSelfReference reports whether `ref` reads the class
// being declared by `decl` from within `decl`'s heritage clause
// (`class C extends C {}`). The class binding is in the TDZ while
// `extends` evaluates, so this is always flagged — regardless of
// the Classes option, which controls hoisting-style cases.
func isClassHeritageSelfReference(decl, ref *wrapperchecker.Node) bool {
	switch decl.Kind() {
	case wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression:
	default:
		return false
	}
	if ref.Pos() < decl.Pos() || ref.End() > decl.End() {
		return false
	}
	// Walk up — only flag when an enclosing HeritageClause exists
	// before we leave `decl`. If we cross a nested function-like or
	// class body first, the reference runs after the declaration
	// completes.
	for cur := ref.Parent(); cur != nil && !cur.Same(decl); cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor,
			wrapperchecker.KindPropertyDeclaration:
			return false
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			// Crossed into a nested class — its initializers run
			// only when an instance is created, by which time decl
			// is fully defined.
			return false
		case wrapperchecker.KindHeritageClause:
			// Only decl's own heritage clause puts decl in TDZ.
			if hp := cur.Parent(); hp != nil && hp.Same(decl) {
				return true
			}
			return false
		}
	}
	return false
}

// isForInOfIterableSelfReference reports whether `ref` reads the loop
// variable being declared by `decl` from the iterable expression of a
// `for (var x in/of x) {}`. The loop variable is `undefined` (var) or
// in TDZ (let/const) at the moment the iterable is read.
func isForInOfIterableSelfReference(decl, ref *wrapperchecker.Node) bool {
	if decl.Kind() != wrapperchecker.KindVariableDeclaration {
		return false
	}
	loop := enclosingForInOrOf(decl)
	if loop == nil {
		return false
	}
	if ref.Pos() < loop.Pos() || ref.End() > loop.End() {
		return false
	}
	// Make sure the ref is not inside the loop body — only the
	// iterable expression of the for-head qualifies as self-init.
	body := loop.IterationBody()
	if body != nil && ref.Pos() >= body.Pos() && ref.End() <= body.End() {
		return false
	}
	return !crossesDeferredBoundary(ref, loop)
}

func enclosingForInOrOf(n *wrapperchecker.Node) *wrapperchecker.Node {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement:
			return cur
		case wrapperchecker.KindSourceFile,
			wrapperchecker.KindBlock,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction:
			return nil
		}
	}
	return nil
}

// inSelfInitRange reports whether `ref` falls inside a TDZ-like
// initializer window for `decl`. For VariableDeclaration / Parameter
// this is simply the node's full range. For BindingElement it is the
// element's own default expression *plus* the enclosing
// VariableDeclaration / Parameter's initializer (the part after the
// binding pattern), but excludes sibling binding elements that
// appear after `decl` in the pattern.
func inSelfInitRange(decl, ref *wrapperchecker.Node) bool {
	switch decl.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter:
		return ref.Pos() >= decl.Pos() && ref.End() <= decl.End()
	case wrapperchecker.KindBindingElement:
		if ref.Pos() >= decl.Pos() && ref.End() <= decl.End() {
			return true
		}
		var pattern, outer *wrapperchecker.Node
		for cur := decl.Parent(); cur != nil; cur = cur.Parent() {
			switch cur.Kind() {
			case wrapperchecker.KindObjectBindingPattern,
				wrapperchecker.KindArrayBindingPattern:
				if pattern == nil {
					pattern = cur
				}
			case wrapperchecker.KindVariableDeclaration,
				wrapperchecker.KindParameter:
				outer = cur
			}
			if outer != nil {
				break
			}
		}
		if outer == nil || pattern == nil {
			return false
		}
		// References after the binding pattern's end are in the
		// outer initializer (TDZ — the destructuring source runs
		// before any element is bound). References between this
		// element's end and the pattern's end are in a sibling
		// element's default expression, which runs after `decl` is
		// bound, so they are NOT TDZ.
		return ref.Pos() >= pattern.End() && ref.End() <= outer.End()
	}
	return false
}

type declKind int

const (
	declOther declKind = iota
	declFunction
	declClass
	declVar
	declLet
	declConst
	declParameter
	declImport
)

func declarationKind(n *wrapperchecker.Node) declKind {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration:
		return declFunction
	case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
		return declClass
	case wrapperchecker.KindParameter:
		return declParameter
	case wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport:
		return declImport
	case wrapperchecker.KindVariableDeclaration:
		return variableDeclarationKind(n)
	case wrapperchecker.KindBindingElement:
		// A BindingElement's storage class is the enclosing
		// VariableDeclaration's. (Parameters are handled separately
		// above.) For parameter destructuring, callers fall into
		// declParameter via the enclosing parameter declaration.
		for cur := n.Parent(); cur != nil; cur = cur.Parent() {
			switch cur.Kind() {
			case wrapperchecker.KindVariableDeclaration:
				return variableDeclarationKind(cur)
			case wrapperchecker.KindParameter:
				return declParameter
			}
		}
		return declOther
	}
	return declOther
}

func variableDeclarationKind(n *wrapperchecker.Node) declKind {
	list := n.Parent()
	if list == nil {
		return declVar
	}
	text := list.SourceText()
	switch {
	case len(text) >= 4 && text[:4] == "var ":
		return declVar
	case len(text) >= 4 && text[:4] == "let ":
		return declLet
	case len(text) >= 6 && text[:6] == "const ":
		return declConst
	}
	return declVar
}

// isFreeReferenceContext reports whether the identifier appears in a
// position where it represents a value reference — not the
// declaration of its own name, not a property access RHS, etc.
func isFreeReferenceContext(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindPropertySignature,
		wrapperchecker.KindEnumMember,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindTypeParameter:
		// These nodes have an identifier *name* plus other children
		// (initializers, types, bodies). Only the name is a declaration;
		// the rest are value/type references that we must still check.
		if name := p.DeclarationName(); name != nil && name.Same(n) {
			return false
		}
		return true
	case wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindExportSpecifier,
		wrapperchecker.KindNamespaceExport,
		wrapperchecker.KindLabeledStatement,
		wrapperchecker.KindBreakStatement,
		wrapperchecker.KindContinueStatement,
		wrapperchecker.KindJsxAttribute:
		return false
	case wrapperchecker.KindPropertyAccessExpression:
		return p.PropertyAccessReceiver().Same(n)
	case wrapperchecker.KindPropertyAssignment:
		if n.Pos() == firstChildPos(p) {
			return false
		}
		return true
	case wrapperchecker.KindShorthandPropertyAssignment:
		gp := p.Parent()
		if gp != nil && gp.Kind() == wrapperchecker.KindObjectBindingPattern {
			return false
		}
		return true
	}
	return true
}

func firstChildPos(p *wrapperchecker.Node) int {
	first := p.FirstChild()
	if first == nil {
		return -1
	}
	return first.Pos()
}
