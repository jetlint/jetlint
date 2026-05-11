// Package nodeprecated implements the no-deprecated rule: flag uses of
// identifiers, members, properties, and signatures whose declarations
// carry a `@deprecated` JSDoc tag.
//
// Behavioral spec ported from typescript-eslint's no-deprecated rule.
// See: https://github.com/typescript-eslint/typescript-eslint/blob/main/packages/eslint-plugin/src/rules/no-deprecated.ts
package nodeprecated

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-deprecated"

// Options is the configurable surface of the rule. Mirrors
// typescript-eslint's `no-deprecated` options.
type Options struct {
	// AllowNames lists identifier names whose deprecation warnings
	// should be suppressed. Mirrors the upstream `allow` option in
	// its bare-string and `{ name: "..." }`-object forms; we don't
	// distinguish source qualifiers (file/package/lib) because the
	// upstream tests we run through use the name alone.
	AllowNames map[string]struct{}
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-deprecated options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allow":
			names, err := parseAllowList(val)
			if err != nil {
				return Options{}, fmt.Errorf("no-deprecated option %q: %w", key, err)
			}
			out.AllowNames = names
		default:
			return Options{}, fmt.Errorf("no-deprecated has no option %q", key)
		}
	}
	return out, nil
}

func parseAllowList(raw json.RawMessage) (map[string]struct{}, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("allow must be an array: %w", err)
	}
	out := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		var s string
		if err := json.Unmarshal(e, &s); err == nil {
			out[s] = struct{}{}
			continue
		}
		var obj struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(e, &obj); err == nil && obj.Name != "" {
			out[obj.Name] = struct{}{}
			continue
		}
	}
	return out, nil
}

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return rule{opts: opts} }

type rule struct{ opts Options }

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier:              r.visitIdentifierMethod,
		wrapperchecker.KindPrivateIdentifier:       r.visitIdentifierMethod,
		wrapperchecker.KindElementAccessExpression: r.visitElementAccessMethod,
		wrapperchecker.KindSuperKeyword:            r.visitSuperKeywordMethod,
	}
}

func (r rule) visitIdentifierMethod(ctx *engine.Context, n *wrapperchecker.Node) {
	visitIdentifier(ctx, n, r.opts)
}
func (r rule) visitElementAccessMethod(ctx *engine.Context, n *wrapperchecker.Node) {
	visitElementAccess(ctx, n, r.opts)
}
func (r rule) visitSuperKeywordMethod(ctx *engine.Context, n *wrapperchecker.Node) {
	visitSuperKeyword(ctx, n, r.opts)
}

// visitSuperKeyword handles `super(...)` constructor calls. The
// resolved signature points at the base class's constructor; flag it
// when that constructor carries `@deprecated`.
func visitSuperKeyword(ctx *engine.Context, n *wrapperchecker.Node, opts Options) {
	parent := n.Parent()
	if parent == nil || parent.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	if !sameNode(parent.CalleeExpression(), n) {
		return
	}
	sig := ctx.Checker().ResolvedSignature(parent)
	if sig == nil || !sig.IsDeprecated() {
		return
	}
	report(ctx, n, "super", sig.DeprecationReason(), opts)
}

func visitIdentifier(ctx *engine.Context, n *wrapperchecker.Node, opts Options) {
	if isInsideImport(n) {
		return
	}
	// Object-destructuring bindings are syntactically declarations (the
	// local binding) but also conceptually a *use* of the source
	// property. We flag the use here before treating the node as a
	// declaration site.
	if checkDestructuringPropertyUse(ctx, n, opts) {
		return
	}
	// JSX attribute names — `<A b={...} />` flags `b` if `A`'s prop
	// type marks the property deprecated. Resolved through the
	// contextual type of the opening element's name, not via the local
	// identifier symbol.
	if checkJsxAttributeUse(ctx, n, opts) {
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
	reportIfDeprecated(ctx, n, opts)
}

// checkJsxAttributeUse handles `<Component prop={x} />`: the `prop`
// identifier is structurally a JsxAttribute name; check it against
// the contextual type of the opening element to flag a deprecated
// prop. Returns true when the visit was handled (reported, or
// confirmed non-deprecated and no further handling needed).
func checkJsxAttributeUse(ctx *engine.Context, n *wrapperchecker.Node, opts Options) bool {
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindJsxAttribute {
		return false
	}
	pp := p.Parent()
	if pp == nil || pp.Kind() != wrapperchecker.KindJsxAttributes {
		return false
	}
	opening := pp.Parent()
	if opening == nil {
		return true
	}
	switch opening.Kind() {
	case wrapperchecker.KindJsxOpeningElement,
		wrapperchecker.KindJsxSelfClosingElement:
	default:
		return true
	}
	tag := jsxOpeningTagName(opening)
	if tag == nil {
		return true
	}
	t := ctx.Checker().ContextualTypeOf(tag)
	if t == nil {
		t = ctx.TypeOf(tag)
		if t == nil {
			return true
		}
	}
	prop := t.PropertySymbol(n.LiteralText())
	if prop == nil || !prop.IsDeprecated() {
		return true
	}
	report(ctx, n, n.LiteralText(), prop.DeprecationReason(), opts)
	return true
}

// jsxOpeningTagName returns the tag-name child of a JsxOpeningElement
// or JsxSelfClosingElement (the component identifier or property
// access used as the JSX tag).
func jsxOpeningTagName(opening *wrapperchecker.Node) *wrapperchecker.Node {
	var name *wrapperchecker.Node
	opening.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPropertyAccessExpression:
			name = c
			return true
		}
		return false
	})
	return name
}

// checkDestructuringPropertyUse reports the deprecation of a source
// property when the identifier is the binding name in an object
// destructuring (`const { b } = a` flags `b` if `a.b` is deprecated).
// Returns true to indicate the visit should stop (we've reported, or
// confirmed the source property is non-deprecated and the identifier
// otherwise carries no use semantics).
func checkDestructuringPropertyUse(ctx *engine.Context, n *wrapperchecker.Node, opts Options) bool {
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
	srcT := destructuringSourceType(ctx, pp)
	if srcT == nil {
		return true
	}
	// Use the property name (`a` in `{ a: b }`) when present; otherwise
	// the binding name (`{ b }` shorthand).
	key := n.LiteralText()
	if pn := p.BindingElementPropertyName(); pn != nil {
		switch pn.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPrivateIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral,
			wrapperchecker.KindNumericLiteral:
			key = pn.LiteralText()
		}
		// If propertyName exists and the binding-name identifier IS our
		// node, the actual lookup happens against the propertyName, not
		// the local. Skip when our node is the local-binding side of a
		// `propName: localName` shape — the propertyName visit covers it.
		if !sameNode(pn, n) {
			return true
		}
	}
	prop := srcT.PropertySymbol(key)
	if prop == nil || !prop.IsDeprecated() {
		return true
	}
	report(ctx, n, key, prop.DeprecationReason(), opts)
	return true
}

// destructuringSourceType resolves the type whose properties are
// being destructured by the given pattern. Walks outward through
// nested BindingElement / ObjectBindingPattern wrappers, looking up
// the property at each level so `const { bar: { anchor } } = x`
// resolves anchor against x.bar.
func destructuringSourceType(ctx *engine.Context, pattern *wrapperchecker.Node) *wrapperchecker.Type {
	parent := pattern.Parent()
	if parent == nil {
		return nil
	}
	switch parent.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		init := parent.VariableDeclarationInitializer()
		if init == nil {
			return nil
		}
		return ctx.TypeOf(init)
	case wrapperchecker.KindBindingElement:
		// Nested: walk to the outer source type, then narrow by the
		// property this BindingElement binds.
		outerPattern := parent.Parent()
		if outerPattern == nil {
			return nil
		}
		outerT := destructuringSourceType(ctx, outerPattern)
		if outerT == nil {
			return nil
		}
		key := bindingElementKeyName(parent)
		if key == "" {
			return nil
		}
		return outerT.PropertyType(key)
	}
	return nil
}

// bindingElementKeyName returns the source-property name a
// BindingElement reads from. Uses the explicit propertyName when set
// (`a: x` reads `a`); falls back to the binding-name identifier for
// shorthand entries (`{ a }` reads `a`).
func bindingElementKeyName(elem *wrapperchecker.Node) string {
	if pn := elem.BindingElementPropertyName(); pn != nil {
		switch pn.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPrivateIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral,
			wrapperchecker.KindNumericLiteral:
			return pn.LiteralText()
		}
		return ""
	}
	name := elem.BindingElementName()
	if name == nil {
		return ""
	}
	if name.Kind() == wrapperchecker.KindIdentifier {
		return name.LiteralText()
	}
	return ""
}


func visitElementAccess(ctx *engine.Context, n *wrapperchecker.Node, opts Options) {
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
	report(ctx, idx, name, prop.DeprecationReason(), opts)
}

func reportIfDeprecated(ctx *engine.Context, n *wrapperchecker.Node, opts Options) {
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
				report(ctx, n, n.LiteralText(), sig.DeprecationReason(), opts)
				return
			}
			// If the signature isn't deprecated but the symbol's
			// deprecation lives at the binding/specifier level (rather
			// than just on one of the function's overloads), flag —
			// the binding-level deprecation applies to every use,
			// including this non-deprecated-overload call.
			if symbolHasBindingLevelDeprecation(sym) {
				report(ctx, n, n.LiteralText(), sym.DeprecationReason(), opts)
			}
			return
		}
	}
	if sym.IsDeprecated() {
		report(ctx, n, n.LiteralText(), sym.DeprecationReason(), opts)
	}
}

func report(ctx *engine.Context, at *wrapperchecker.Node, name, reason string, opts Options) {
	if name == "" {
		name = at.LiteralText()
	}
	if _, allowed := opts.AllowNames[name]; allowed {
		return
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

// symbolHasBindingLevelDeprecation reports whether the symbol's
// deprecation is at the binding site (import/export specifier,
// variable, class member, etc.) rather than at one specific
// function/method overload. Overload-level deprecation is handled
// by the resolved-signature check; surfacing the alias-chain's
// any-overload result there would over-report when the selected
// overload is fine.
func symbolHasBindingLevelDeprecation(sym *wrapperchecker.Symbol) bool {
	d := sym.FirstDeprecatedDeclaration()
	if d == nil {
		return false
	}
	switch d.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindMethodSignature,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindCallSignature,
		wrapperchecker.KindConstructSignature:
		return false
	}
	return true
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
