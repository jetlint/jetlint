// Package nounnecessarytypearguments implements the
// no-unnecessary-type-arguments rule: flag explicit type arguments
// that match the type parameter's declared default, e.g. `f<number>()`
// when `f` is `function f<T = number>(): void`.
//
// Behavioral spec ported from typescript-eslint's no-unnecessary-type-
// arguments rule. See:
// https://github.com/typescript-eslint/typescript-eslint/blob/main/packages/eslint-plugin/src/rules/no-unnecessary-type-arguments.ts
package nounnecessarytypearguments

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unnecessary-type-arguments"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:              visit,
		wrapperchecker.KindNewExpression:               visit,
		wrapperchecker.KindTaggedTemplateExpression:    visit,
		wrapperchecker.KindTypeReference:               visit,
		wrapperchecker.KindExpressionWithTypeArguments: visit,
		wrapperchecker.KindJsxOpeningElement:           visit,
		wrapperchecker.KindJsxSelfClosingElement:       visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// `f<number>` used as a value (an "instantiation expression",
	// TS 4.7+) parses to ExpressionWithTypeArguments without a
	// HeritageClause parent. TypeScript does not apply default type
	// parameters to instantiation expressions, so explicit type
	// arguments are always meaningful there — don't flag.
	if n.Kind() == wrapperchecker.KindExpressionWithTypeArguments {
		if p := n.Parent(); p == nil || p.Kind() != wrapperchecker.KindHeritageClause {
			return
		}
	}
	args := n.TypeArgumentNodes()
	if len(args) == 0 {
		return
	}
	params := typeParametersForNode(ctx, n)
	if params == nil {
		return
	}
	// Only check the last type argument — earlier ones are required to
	// reach later ones, so they can never be "unnecessary".
	i := len(args) - 1
	if i >= len(params) {
		return
	}
	def := params[i].TypeParameterDefaultType()
	if def == nil {
		return
	}
	defaultType := ctx.Checker().TypeOf(def)
	argType := ctx.Checker().TypeOf(args[i])
	if defaultType == nil || argType == nil {
		return
	}
	if !typesMatch(defaultType, argType) {
		return
	}
	ctx.Report(args[i], "this is the default value for this type parameter, so it can be omitted")
}

// typeParametersForNode resolves the type-parameter declarations that
// govern this node's type arguments.
func typeParametersForNode(ctx *engine.Context, n *wrapperchecker.Node) []*wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindCallExpression,
		wrapperchecker.KindNewExpression,
		wrapperchecker.KindTaggedTemplateExpression,
		wrapperchecker.KindJsxOpeningElement,
		wrapperchecker.KindJsxSelfClosingElement:
		if sig := ctx.Checker().ResolvedSignatureGeneral(n); sig != nil {
			if decl := sig.SignatureDeclaration(); decl != nil {
				if tps := decl.TypeParameterDeclarations(); len(tps) > 0 {
					return tps
				}
			}
		}
		// new C() with no resolved sig — fall back to the class'
		// type parameters via its symbol.
		if n.Kind() == wrapperchecker.KindNewExpression {
			return typeParametersFromSymbolOf(ctx, n.CalleeExpression(), false)
		}
		return nil
	case wrapperchecker.KindTypeReference:
		// Type-reference position is always a type context.
		return typeParametersFromSymbolOf(ctx, n.TypeReferenceTypeName(), true)
	case wrapperchecker.KindExpressionWithTypeArguments:
		// `extends X` is a value context (resolves to the runtime
		// constructor), `implements X` is a type context.
		typeContext := true
		if p := n.Parent(); p != nil && p.HeritageClauseToken() == wrapperchecker.KindExtendsKeyword {
			// `extends` on a class is value context; on an interface
			// it's type context. tsgo's HeritageClause node knows the
			// keyword but not the parent kind here — use the
			// grandparent.
			if gp := p.Parent(); gp != nil && gp.Kind() == wrapperchecker.KindClassDeclaration ||
				gp != nil && gp.Kind() == wrapperchecker.KindClassExpression {
				typeContext = false
			}
		}
		return typeParametersFromSymbolOf(ctx, n.ExpressionWithTypeArgumentsExpression(), typeContext)
	}
	return nil
}

// typeParametersFromSymbolOf resolves the type-parameter declarations
// of the symbol referenced by `name`. When a symbol has multiple
// declarations from declaration merging (e.g. a class merged with an
// interface, both declaring the same name with different defaults), the
// ordering matters: in a type context (e.g. `implements`), interface/
// type-alias declarations are preferred; in a value context (e.g.
// `extends` on a class, or `new`), class/variable declarations are
// preferred. For variable declarations carrying construct signatures,
// the type parameters come from the construct signature.
func typeParametersFromSymbolOf(ctx *engine.Context, name *wrapperchecker.Node, typeContext bool) []*wrapperchecker.Node {
	if name == nil {
		return nil
	}
	sym := ctx.Checker().SymbolOf(name)
	if sym == nil {
		return nil
	}
	decls := sym.Declarations()
	sorted := sortDeclarations(decls, typeContext)
	for _, decl := range sorted {
		if tps := decl.TypeParameterDeclarations(); len(tps) > 0 {
			return tps
		}
		if decl.Kind() == wrapperchecker.KindVariableDeclaration {
			// `declare var Foo: { new <T>(...) }` — the type parameters
			// live on the construct signature of the variable's type.
			if t := ctx.Checker().TypeOfSymbol(sym); t != nil {
				for _, sig := range t.ConstructSignatures() {
					if d := sig.SignatureDeclaration(); d != nil {
						if tps := d.TypeParameterDeclarations(); len(tps) > 0 {
							return tps
						}
					}
				}
			}
		}
	}
	return nil
}

func isTypeContextDeclaration(d *wrapperchecker.Node) bool {
	switch d.Kind() {
	case wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration:
		return true
	}
	return false
}

func sortDeclarations(decls []*wrapperchecker.Node, typeContext bool) []*wrapperchecker.Node {
	out := make([]*wrapperchecker.Node, len(decls))
	copy(out, decls)
	// Stable sort: type-context decls first.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && isTypeContextDeclaration(out[j]) && !isTypeContextDeclaration(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if !typeContext {
		// Reverse: value-context decls first.
		for i, k := 0, len(out)-1; i < k; i, k = i+1, k-1 {
			out[i], out[k] = out[k], out[i]
		}
	}
	return out
}

// typesMatch mirrors the upstream comparison: identity first
// (TypeScript caches and de-duplicates types) and a structural fallback
// for aliased generic shapes — where the alias does not produce a
// shared type instance (`type A = Map<string, string>` vs literal
// `Map<string, string>`). For the structural case we require both
// types to have the same number of type arguments, all of which are
// pointer-identical. This covers the common alias-of-generic case
// without invoking the full structural-equivalence machinery.
func typesMatch(a, b *wrapperchecker.Type) bool {
	if a.Identical(b) {
		return true
	}
	aArgs := safeTypeArguments(a)
	bArgs := safeTypeArguments(b)
	if len(aArgs) == 0 || len(aArgs) != len(bArgs) {
		return false
	}
	for i := range aArgs {
		if !aArgs[i].Identical(bArgs[i]) {
			return false
		}
	}
	return true
}

// safeTypeArguments returns the type's type arguments, or nil if the
// type has none or the underlying checker rejects the query. The
// wrapper's TypeArguments() filters non-Object types and non-
// TypeReferences but still propagates panics for TypeReferences whose
// target lacks the expected InterfaceType backing (e.g., aliased
// generic signatures like `React.FC<P>` — jetlint#625).
func safeTypeArguments(t *wrapperchecker.Type) (args []*wrapperchecker.Type) {
	if t == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			args = nil
		}
	}()
	return t.TypeArguments()
}
