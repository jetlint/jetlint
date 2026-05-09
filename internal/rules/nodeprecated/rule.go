// Package nodeprecated implements the no-deprecated rule: flag uses of
// identifiers, members, properties, and signatures whose declarations
// carry a `@deprecated` JSDoc tag.
//
// Behavioral spec ported from typescript-eslint's no-deprecated rule.
// See: https://github.com/typescript-eslint/typescript-eslint/blob/main/packages/eslint-plugin/src/rules/no-deprecated.ts
package nodeprecated

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-deprecated"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier:              visitIdentifier,
		wrapperchecker.KindPrivateIdentifier:       visitIdentifier,
		wrapperchecker.KindElementAccessExpression: visitElementAccess,
	}
}

func visitIdentifier(ctx *engine.Context, n *wrapperchecker.Node) {
	if isInsideImport(n) {
		return
	}
	// Object-destructuring bindings are syntactically declarations (the
	// local binding) but also conceptually a *use* of the source
	// property. We flag the use here before treating the node as a
	// declaration site.
	if checkDestructuringPropertyUse(ctx, n) {
		return
	}
	if isDeclarationSite(n) {
		return
	}
	if p := n.Parent(); p != nil {
		switch p.Kind() {
		case wrapperchecker.KindElementAccessExpression:
			// `a[b]` — the index identifier `b` is a use of the local
			// `b`, not a property of `a`. Don't skip; check the symbol.
		case wrapperchecker.KindJsxClosingElement:
			return
		}
	}
	reportIfDeprecated(ctx, n)
}

// checkDestructuringPropertyUse reports the deprecation of a source
// property when the identifier is the binding name in an object
// destructuring (`const { b } = a` flags `b` if `a.b` is deprecated).
// Returns true to indicate the visit should stop (we've reported, or
// confirmed the source property is non-deprecated and the identifier
// otherwise carries no use semantics).
func checkDestructuringPropertyUse(ctx *engine.Context, n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindBindingElement {
		return false
	}
	// Skip if this is the initializer (a use), not the binding name.
	if init := p.BindingElementInitializer(); init != nil && sameNode(init, n) {
		return false
	}
	pp := p.Parent()
	if pp == nil || pp.Kind() != wrapperchecker.KindObjectBindingPattern {
		return false
	}
	source := destructuringSource(pp)
	if source == nil {
		return true
	}
	srcT := ctx.TypeOf(source)
	if srcT == nil {
		return true
	}
	prop := srcT.PropertySymbol(n.LiteralText())
	if prop == nil || !prop.IsDeprecated() {
		return true
	}
	report(ctx, n, n.LiteralText(), prop.DeprecationReason())
	return true
}

// destructuringSource walks up from the binding pattern to the
// expression whose value is being destructured (the right-hand side of
// `const { ... } = expr`). Returns nil for parameters or other
// destructuring-without-source contexts.
func destructuringSource(pattern *wrapperchecker.Node) *wrapperchecker.Node {
	cur := pattern.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindVariableDeclaration:
			return cur.VariableDeclarationInitializer()
		case wrapperchecker.KindBindingElement:
			// Nested: `const { bar: { anchor } } = x` — the outer
			// BindingElement carries the source via its property name's
			// type; we use its parent's source recursively.
			return destructuringNestedSource(cur)
		}
		cur = cur.Parent()
	}
	return nil
}

func destructuringNestedSource(_ *wrapperchecker.Node) *wrapperchecker.Node {
	// Nested destructuring requires resolving the property type of an
	// outer source — too many wrapper-API gaps to implement cleanly
	// today. Returning nil leaves nested cases unhandled.
	return nil
}

func visitElementAccess(ctx *engine.Context, n *wrapperchecker.Node) {
	idx := n.ElementAccessIndex()
	recv := n.ElementAccessReceiver()
	if idx == nil || recv == nil {
		return
	}
	// Resolve the index expression to a string-literal type, if it
	// reduces to one (e.g. `'b'`, a `const key = 'b'`, a template
	// literal `${...}` that always concatenates known strings).
	idxT := ctx.TypeOf(idx)
	if idxT == nil {
		return
	}
	name, ok := idxT.StringLiteralValue()
	if !ok {
		return
	}
	objT := ctx.TypeOf(recv)
	if objT == nil {
		return
	}
	prop := objT.PropertySymbol(name)
	if prop == nil || !prop.IsDeprecated() {
		return
	}
	report(ctx, idx, name, prop.DeprecationReason())
}

func reportIfDeprecated(ctx *engine.Context, n *wrapperchecker.Node) {
	sym := ctx.Checker().SymbolOf(n)
	// Shorthand property `{ x }` resolves SymbolOf to the new object's
	// property; the deprecation actually attaches to the value-binding
	// symbol (the local `x`). Prefer that when the parent is a
	// ShorthandPropertyAssignment.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindShorthandPropertyAssignment {
		if v := ctx.Checker().ShorthandAssignmentValueSymbol(p); v != nil {
			sym = v
		}
	}
	if sym == nil {
		return
	}
	// For call-like uses, prefer the resolved signature's deprecation
	// (an overloaded function may have only certain signatures
	// deprecated).
	callLike := getCallLikeNode(n)
	if callLike != nil {
		if sig := ctx.Checker().ResolvedSignatureGeneral(callLike); sig != nil {
			if sig.IsDeprecated() {
				report(ctx, n, n.LiteralText(), sig.DeprecationReason())
				return
			}
			// If the signature isn't deprecated but the symbol's
			// non-method declaration is, still flag.
			if sym.IsDeprecated() && !symbolHasMethodOrFunctionDecl(sym) {
				report(ctx, n, n.LiteralText(), sym.DeprecationReason())
			}
			return
		}
	}
	if sym.IsDeprecated() {
		report(ctx, n, n.LiteralText(), sym.DeprecationReason())
	}
}

func report(ctx *engine.Context, at *wrapperchecker.Node, name, reason string) {
	if name == "" {
		name = at.LiteralText()
	}
	if reason == "" {
		ctx.Report(at, fmt.Sprintf("`%s` is deprecated.", name))
	} else {
		ctx.Report(at, fmt.Sprintf("`%s` is deprecated. %s", name, reason))
	}
}

func isDeclarationSite(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	// For named-decl parents, the identifier is a declaration only when
	// it's the *name* slot of that decl. We approximate by checking the
	// first identifier child of the parent — declarations put the name
	// before the type/initializer/body in the source order.
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindEnumMember,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindMethodSignature,
		wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindPropertySignature,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindModuleDeclaration,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindNamespaceExport,
		wrapperchecker.KindImportEqualsDeclaration:
		return isFirstNameChild(p, n)
	case wrapperchecker.KindArrayBindingPattern,
		wrapperchecker.KindObjectBindingPattern,
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindExportSpecifier:
		return true
	case wrapperchecker.KindBindingElement:
		// `{ a: c = b } = ...` — name and (optional) propertyName are
		// declarations; the initializer is a use.
		if init := p.BindingElementInitializer(); init != nil && sameNode(init, n) {
			return false
		}
		return true
	case wrapperchecker.KindPropertyAssignment:
		// `{ foo: bar }` — `foo` is the key (declaration), `bar` is
		// the value (use).
		return isFirstNameChild(p, n)
	}
	return false
}

func isFirstNameChild(parent, child *wrapperchecker.Node) bool {
	var first *wrapperchecker.Node
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier ||
			c.Kind() == wrapperchecker.KindPrivateIdentifier {
			first = c
			return true
		}
		return false
	})
	return first != nil && sameNode(first, child)
}


func isInsideImport(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindImportDeclaration,
			wrapperchecker.KindImportEqualsDeclaration:
			return true
		case wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindInterfaceDeclaration,
			wrapperchecker.KindBlock,
			wrapperchecker.KindVariableDeclaration,
			wrapperchecker.KindUnionType:
			return false
		}
	}
	return false
}

// getCallLikeNode walks up through MemberExpression chains to find the
// CallExpression / NewExpression / TaggedTemplateExpression /
// JsxOpeningElement / JsxSelfClosingElement that has this node as its
// callee. Returns nil when the node is not in callee position.
func getCallLikeNode(n *wrapperchecker.Node) *wrapperchecker.Node {
	cur := n
	for {
		p := cur.Parent()
		if p == nil {
			return nil
		}
		// Walk through property accesses where we're the property side.
		if p.Kind() == wrapperchecker.KindPropertyAccessExpression {
			if recv := p.PropertyAccessReceiver(); recv == nil || sameNode(recv, cur) {
				return nil
			}
			cur = p
			continue
		}
		switch p.Kind() {
		case wrapperchecker.KindCallExpression,
			wrapperchecker.KindNewExpression:
			if callee := p.CalleeExpression(); callee != nil && sameNode(callee, cur) {
				return p
			}
			return nil
		case wrapperchecker.KindTaggedTemplateExpression,
			wrapperchecker.KindJsxOpeningElement,
			wrapperchecker.KindJsxSelfClosingElement:
			return p
		}
		return nil
	}
}

func symbolHasMethodOrFunctionDecl(sym *wrapperchecker.Symbol) bool {
	for _, d := range sym.Declarations() {
		switch d.Kind() {
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodSignature:
			return true
		}
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
