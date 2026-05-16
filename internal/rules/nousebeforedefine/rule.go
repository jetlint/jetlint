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

// kindClassStaticBlockDeclaration mirrors `ast.KindClassStaticBlockDeclaration`,
// which is not re-exported by the checker wrapper. Static block bodies
// run eagerly as part of class evaluation.
const kindClassStaticBlockDeclaration wrapperchecker.Kind = 176

// kindWithStatement mirrors `ast.KindWithStatement`. References inside
// a `with` block fail TS-go symbol resolution (the object's properties
// are not statically known), so we fall back to a source-file scope
// lookup for these.
const kindWithStatement wrapperchecker.Kind = 255

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
	if r.opts.IgnoreTypeReferences && isInTypePosition(n) {
		return
	}
	if r.opts.AllowNamedExports && isExportSpecifierLocal(n) {
		return
	}
	sym := ctx.Checker().SymbolOf(n)
	if sym == nil {
		// TS-go's resolver cannot reach through a `with` block (the
		// object's property set is dynamic). Fall back to a
		// source-file scope lookup so a let/const/etc. declared
		// after the with statement is still seen.
		if isInWithStatement(n) {
			if local := findSourceFileBinding(n, n.LiteralText()); local != nil && n.Pos() < local.Pos() {
				ctx.Report(n, "'"+n.SourceText()+"' was used before it was defined.")
			}
		}
		return
	}
	decls := sym.Declarations()
	// `export { a }` resolves to a synthetic ExportSpecifier symbol
	// whose only declaration is itself — TS-go does not auto-merge
	// the local binding. Look it up in the source-file scope so the
	// position comparison sees the real declaration.
	if isExportSpecifierLocal(n) && allDeclsAreExportSpecifiers(decls) {
		if local := findSourceFileBinding(n, n.LiteralText()); local != nil {
			decls = []*wrapperchecker.Node{local}
		}
	}
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
		// ExportSpecifiers are re-export bindings that mirror the
		// real local binding; the real declaration is what counts
		// for use-before-define.
		if d.Kind() == wrapperchecker.KindExportSpecifier {
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
	// References inside an eager class-evaluation context (heritage
	// clause, static field initializer, computed name) are always
	// TDZ-sensitive and reported regardless of the hoisting options.
	eager := referenceInEagerClassContext(n)
	switch declarationKind(first) {
	case declFunction:
		if !r.opts.Functions && !eager {
			return
		}
	case declClass:
		// `new ClassName()` outside a deferred body requires the
		// class binding to be initialized; oxlint flags those even
		// when Classes: false.
		newCtor := isNewExpressionCallee(n) && !referenceInDeferredBody(n)
		if !r.opts.Classes && !eager && !newCtor {
			return
		}
	case declVar:
		// `var` declarations are always hoisted; oxlint flags
		// use-before-define for `var` regardless of the Variables
		// option except when the reference sits in a deferred body
		// that is *outside* the variable's own scope (the body may
		// execute long after the var is fully bound).
		if !r.opts.Variables && !eager &&
			referenceInDeferredBody(n) &&
			!sameDeferredBody(n, first) {
			return
		}
	case declLet, declConst:
		// `Variables: false` suppresses lexical-binding
		// use-before-define when the reference sits in a deferred
		// body and is not part of an eager class-evaluation chain.
		if !r.opts.Variables && !eager && referenceInDeferredBody(n) {
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
// reports whether the walk passes through a deferred body. Function,
// method, arrow, accessor, and constructor bodies are deferred; class
// heritage clauses and method/property computed names are eager. Once
// we have established that we are on an eager path through a class
// member (via a computed name), traversing the surrounding class
// declaration is not a deferred boundary.
func crossesDeferredBoundary(ref, decl *wrapperchecker.Node) bool {
	prev := ref
	eagerThroughClass := false
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
			if prev.Kind() == wrapperchecker.KindComputedPropertyName {
				eagerThroughClass = true
				break
			}
			// A static field's initializer runs eagerly at
			// class-create time — while the class binding is still
			// in the TDZ — so it counts as an eager path. Instance
			// fields run on construction, well after the class is
			// fully bound, so they remain deferred.
			if cur.Kind() == wrapperchecker.KindPropertyDeclaration && cur.HasStaticModifier() {
				eagerThroughClass = true
				break
			}
			return true
		case kindClassStaticBlockDeclaration:
			// Class static blocks run eagerly during class
			// evaluation, mirroring static field initializers.
			eagerThroughClass = true
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			if eagerThroughClass {
				// Reset for any further enclosing class — each class
				// nesting level resolves separately.
				eagerThroughClass = false
				break
			}
			if prev.Kind() != wrapperchecker.KindHeritageClause {
				return true
			}
		}
		prev = cur
	}
	return false
}

func allDeclsAreExportSpecifiers(decls []*wrapperchecker.Node) bool {
	if len(decls) == 0 {
		return false
	}
	for _, d := range decls {
		if d.Kind() != wrapperchecker.KindExportSpecifier {
			return false
		}
	}
	return true
}

// findSourceFileBinding walks up to the enclosing SourceFile and
// scans its top-level statements for a binding whose name matches
// `name` — used as a fallback when TS-go's SymbolOf gives us a
// synthetic re-export symbol that does not link to the local
// declaration.
func findSourceFileBinding(ref *wrapperchecker.Node, name string) *wrapperchecker.Node {
	if name == "" {
		return nil
	}
	var sourceFile *wrapperchecker.Node
	for cur := ref.Parent(); cur != nil; cur = cur.Parent() {
		if cur.Kind() == wrapperchecker.KindSourceFile {
			sourceFile = cur
			break
		}
	}
	if sourceFile == nil {
		return nil
	}
	var found *wrapperchecker.Node
	sourceFile.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if found != nil {
			return true
		}
		switch stmt.Kind() {
		case wrapperchecker.KindVariableStatement:
			if list := stmt.VariableStatementDeclarationList(); list != nil {
				list.ForEachChild(func(decl *wrapperchecker.Node) bool {
					if decl.Kind() == wrapperchecker.KindVariableDeclaration {
						if nm := decl.DeclarationName(); nm != nil && nm.LiteralText() == name {
							found = decl
						}
					}
					return found != nil
				})
			}
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindEnumDeclaration,
			wrapperchecker.KindModuleDeclaration:
			if nm := stmt.DeclarationName(); nm != nil && nm.LiteralText() == name {
				found = stmt
			}
		}
		return found != nil
	})
	return found
}

// isInWithStatement reports whether `n` is nested inside a `with`
// statement.
func isInWithStatement(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		if cur.Kind() == kindWithStatement {
			return true
		}
		if cur.Kind() == wrapperchecker.KindSourceFile {
			return false
		}
	}
	return false
}

// isNewExpressionCallee reports whether `n` is the constructor
// expression of a `new X(...)` expression (or a parenthesized form).
func isNewExpressionCallee(n *wrapperchecker.Node) bool {
	cur := n
	for cur != nil {
		p := cur.Parent()
		if p == nil {
			return false
		}
		switch p.Kind() {
		case wrapperchecker.KindParenthesizedExpression:
			cur = p
			continue
		case wrapperchecker.KindNewExpression:
			callee := p.FirstChild()
			return callee != nil && callee.Same(cur)
		}
		return false
	}
	return false
}

// sameDeferredBody reports whether `ref` and `decl` share the same
// enclosing deferred body (function / method / accessor / constructor).
// When they do, the body runs the declaration's initialization before
// any caller can observe the binding state — hoisting rules apply as
// if at the top level.
func sameDeferredBody(ref, decl *wrapperchecker.Node) bool {
	rb := enclosingDeferredBody(ref)
	db := enclosingDeferredBody(decl)
	if rb == nil && db == nil {
		return true
	}
	if rb == nil || db == nil {
		return false
	}
	return rb.Same(db)
}

func enclosingDeferredBody(n *wrapperchecker.Node) *wrapperchecker.Node {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return cur
		case wrapperchecker.KindSourceFile:
			return nil
		}
	}
	return nil
}

// referenceInDeferredBody reports whether `ref` sits inside a body
// that runs deferred: function / method / accessor / constructor body,
// or an instance class field initializer. Used to be optimistic about
// forward references that may execute after the surrounding bindings
// complete (in oxlint-compatible mode).
func referenceInDeferredBody(ref *wrapperchecker.Node) bool {
	for cur := ref.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return true
		case wrapperchecker.KindPropertyDeclaration:
			if !cur.HasStaticModifier() {
				return true
			}
		case kindClassStaticBlockDeclaration:
			return true
		case wrapperchecker.KindSourceFile:
			return false
		}
	}
	return false
}

// enclosingClass returns the nearest ClassDeclaration / ClassExpression
// that contains `n`, or nil if there is none in the same compilation
// unit.
func enclosingClass(n *wrapperchecker.Node) *wrapperchecker.Node {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			return cur
		case wrapperchecker.KindSourceFile:
			return nil
		}
	}
	return nil
}

// referenceInEagerClassContext reports whether `ref` sits inside an
// eagerly-evaluated position of a class definition — a heritage
// clause, a computed property name, or a static field initializer —
// AND the path from `ref` up to the source file never crosses a
// deferred boundary (a function-like body or an instance-field
// initializer). Eager-class references in this sense evaluate while
// some class binding is still in TDZ, so use-before-define applies
// regardless of the hoisting options.
func referenceInEagerClassContext(ref *wrapperchecker.Node) bool {
	sawEager := false
	prev := ref
	for cur := ref.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return false
		case wrapperchecker.KindMethodDeclaration:
			if prev.Kind() != wrapperchecker.KindComputedPropertyName {
				return false
			}
			sawEager = true
		case wrapperchecker.KindPropertyDeclaration:
			if prev.Kind() == wrapperchecker.KindComputedPropertyName {
				sawEager = true
				break
			}
			if cur.HasStaticModifier() {
				sawEager = true
				break
			}
			return false
		case wrapperchecker.KindHeritageClause:
			sawEager = true
		case kindClassStaticBlockDeclaration:
			// Static blocks run eagerly during class evaluation —
			// the enclosing class's binding is live, but any outer
			// class binding is still in TDZ.
			sawEager = true
		case wrapperchecker.KindSourceFile:
			return sawEager
		}
		prev = cur
	}
	return sawEager
}

// isExportSpecifierLocal reports whether `n` is the local-reference
// identifier of an ExportSpecifier (`export { a }` or the `a` in
// `export { a as b }`).
func isExportSpecifierLocal(n *wrapperchecker.Node) bool {
	p := n.Parent()
	return p != nil && p.Kind() == wrapperchecker.KindExportSpecifier
}

// isInTypePosition reports whether the identifier `n` sits inside a
// TypeScript type-only construct (TypeReference, TypeQuery, TypeLiteral,
// FunctionType, InterfaceDeclaration, TypeAliasDeclaration). Values
// inside these constructs are erased at runtime, so a value-level
// "use before define" doesn't apply.
func isInTypePosition(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindTypeReference,
			wrapperchecker.KindTypeQuery,
			wrapperchecker.KindTypeLiteral,
			wrapperchecker.KindFunctionType,
			wrapperchecker.KindInterfaceDeclaration,
			wrapperchecker.KindTypeAliasDeclaration:
			return true
		case wrapperchecker.KindSourceFile,
			wrapperchecker.KindBlock,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			return false
		}
	}
	return false
}

// isClassHeritageSelfReference reports whether `ref` reads the class
// being declared by `decl` from within `decl`'s heritage clause
// (`class C extends C {}`) or from a computed method / field name
// (`class C { [C](){} }`). Both run eagerly during class evaluation,
// before the class binding becomes live, so we flag regardless of the
// Classes option.
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
	var sawComputedName bool
	prev := ref
	eagerThroughClass := false
	for cur := ref.Parent(); cur != nil && !cur.Same(decl); cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return false
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindPropertyDeclaration:
			if prev.Kind() == wrapperchecker.KindComputedPropertyName {
				sawComputedName = true
				eagerThroughClass = true
				break
			}
			// A static field initializer is eager relative to its
			// OUTER classes (those whose binding is still in TDZ)
			// but NOT to its own class (whose binding has been
			// initialized before its static elements run). Treat as
			// eager only when this static field's owning class is a
			// nested inner class, distinct from `decl`.
			if cur.Kind() == wrapperchecker.KindPropertyDeclaration &&
				cur.HasStaticModifier() {
				if owner := enclosingClass(cur); owner != nil && !owner.Same(decl) {
					sawComputedName = true
					eagerThroughClass = true
					break
				}
			}
			return false
		case kindClassStaticBlockDeclaration:
			// Static blocks of inner classes are eager wrt decl; a
			// static block of decl itself is not — by the time it
			// runs, decl's class binding is initialized.
			if owner := enclosingClass(cur); owner != nil && !owner.Same(decl) {
				sawComputedName = true
				eagerThroughClass = true
				break
			}
			return false
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			if eagerThroughClass {
				eagerThroughClass = false
				break
			}
			return false
		case wrapperchecker.KindHeritageClause:
			if hp := cur.Parent(); hp != nil && hp.Same(decl) {
				return true
			}
			return false
		}
		prev = cur
	}
	return sawComputedName
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
	case wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindModuleDeclaration,
		wrapperchecker.KindTypeAliasDeclaration:
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
		wrapperchecker.KindNamespaceExport,
		wrapperchecker.KindLabeledStatement,
		wrapperchecker.KindBreakStatement,
		wrapperchecker.KindContinueStatement,
		wrapperchecker.KindJsxAttribute:
		return false
	case wrapperchecker.KindExportSpecifier:
		// `export { a }` — the identifier `a` references the value
		// being exported. It IS a use-before-define when the local
		// declaration of `a` comes later in source, unless the user
		// opted into AllowNamedExports.
		return true
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
