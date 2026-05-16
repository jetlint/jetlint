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
		if isSelfInitReference(first, n) {
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
// not nested in a deferred function/method/class body.
func isSelfInitReference(decl, ref *wrapperchecker.Node) bool {
	switch decl.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement:
	default:
		return false
	}
	if ref.Pos() < decl.Pos() || ref.End() > decl.End() {
		return false
	}
	for cur := ref.Parent(); cur != nil && cur != decl; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			return false
		}
	}
	return true
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
	return declOther
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
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindExportSpecifier,
		wrapperchecker.KindNamespaceExport,
		wrapperchecker.KindTypeParameter,
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
