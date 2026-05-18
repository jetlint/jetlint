// Package noredeclare implements the no-redeclare rule: a binding
// name introduced in a scope must not be introduced again in the same
// scope. Catches `var a; var a;`, `function a(){} function a(){}`,
// `var a; function a(){}`, parameter-vs-var shadowing, duplicate type
// aliases, and TypeScript declaration mergings that aren't legal
// (class+enum, type-alias+interface, etc.).
//
// Two passes:
//
//  1. Symbol-based — TypeScript's binder merges legal mergings into a
//     single Symbol; we look at each Symbol's declarations and ask
//     whether any pair is a real conflict. This catches type/value
//     slot collisions (class+enum, type-alias+interface) and
//     same-scope var-redeclare where TS picks the same symbol.
//
//  2. Sloppy-mode hoisting — TypeScript treats every FunctionDeclaration
//     as block-scoped regardless of strict mode, so it gives separate
//     symbols for `var a; function a(){}` and for two `function foo`s
//     across switch case clauses. Biome's no-redeclare follows the JS
//     spec: in sloppy mode (no `"use strict"`, not an ES module) those
//     function decls hoist to the enclosing function scope. We do our
//     own per-scope name accounting to catch those.
package noredeclare

import (
	"fmt"
	"sort"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-redeclare"

// New constructs the rule with default options. The rule has no
// configurable options today.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, sf *wrapperchecker.Node) {
	reported := map[[2]int]bool{}
	report := func(n *wrapperchecker.Node, name string) {
		key := [2]int{n.Pos(), n.End()}
		if reported[key] {
			return
		}
		reported[key] = true
		ctx.Report(n, fmt.Sprintf("'%s' is already defined.", name))
	}
	symbolPass(ctx, sf, report)
	hoistPass(sf, report)
}

// --- symbol-based pass ---------------------------------------------

type declKind int

const (
	dkUnknown declKind = iota
	dkVar
	dkFuncImpl
	dkFuncOverload
	dkClass
	dkEnum
	dkInterface
	dkTypeAlias
	dkNamespace
	dkParam
	dkBindingElt
	dkImport
)

func symbolPass(ctx *engine.Context, sf *wrapperchecker.Node, report func(*wrapperchecker.Node, string)) {
	chk := ctx.Checker()
	seen := map[uintptr]bool{}
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		name := bindingNameOf(n)
		if name != nil && name.Kind() == wrapperchecker.KindIdentifier {
			sym := chk.SymbolOf(name)
			if sym != nil && !seen[sym.ID()] {
				seen[sym.ID()] = true
				if sym.Name() != "default" {
					if reportAt := findRedeclare(sym); reportAt != nil {
						report(reportAt, sym.Name())
					}
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(sf)
}

func bindingNameOf(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		return n.VariableDeclarationName()
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindModuleDeclaration,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindImportSpecifier:
		return firstIdentChild(n)
	case wrapperchecker.KindParameter:
		return n.ParameterName()
	case wrapperchecker.KindBindingElement:
		bn := n.BindingElementName()
		if bn != nil && bn.Kind() == wrapperchecker.KindIdentifier {
			return bn
		}
	}
	return nil
}

func firstIdentChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if found != nil {
			return false
		}
		if c.Kind() == wrapperchecker.KindIdentifier {
			found = c
			return true
		}
		return false
	})
	return found
}

func classify(d *wrapperchecker.Node) declKind {
	if d == nil {
		return dkUnknown
	}
	switch d.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		return dkVar
	case wrapperchecker.KindFunctionDeclaration:
		if d.FunctionBody() != nil {
			return dkFuncImpl
		}
		return dkFuncOverload
	case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
		return dkClass
	case wrapperchecker.KindEnumDeclaration:
		return dkEnum
	case wrapperchecker.KindInterfaceDeclaration:
		return dkInterface
	case wrapperchecker.KindTypeAliasDeclaration:
		return dkTypeAlias
	case wrapperchecker.KindModuleDeclaration:
		return dkNamespace
	case wrapperchecker.KindParameter:
		return dkParam
	case wrapperchecker.KindBindingElement:
		return dkBindingElt
	case wrapperchecker.KindImportClause,
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindNamespaceImport:
		return dkImport
	}
	return dkUnknown
}

func findRedeclare(sym *wrapperchecker.Symbol) *wrapperchecker.Node {
	decls := sym.Declarations()
	if len(decls) < 2 {
		return nil
	}
	type info struct {
		node *wrapperchecker.Node
		kind declKind
	}
	infos := make([]info, 0, len(decls))
	for _, d := range decls {
		k := classify(d)
		if k == dkUnknown {
			continue
		}
		infos = append(infos, info{node: d, kind: k})
	}
	if len(infos) < 2 {
		return nil
	}
	sort.SliceStable(infos, func(i, j int) bool {
		return infos[i].node.Pos() < infos[j].node.Pos()
	})
	for i := 1; i < len(infos); i++ {
		for j := 0; j < i; j++ {
			if conflicts(infos[j].kind, infos[i].kind) {
				return infos[i].node
			}
		}
	}
	return nil
}

// conflicts reports whether two declaration kinds of the same symbol
// constitute a redeclaration (vs a legal merge).
func conflicts(a, b declKind) bool {
	if a > b {
		a, b = b, a
	}
	// Namespace augmentation merges with anything.
	if a == dkNamespace || b == dkNamespace {
		return false
	}
	switch a {
	case dkVar:
		switch b {
		case dkVar, dkFuncImpl, dkFuncOverload, dkBindingElt, dkClass, dkEnum, dkParam:
			return true
		}
	case dkFuncImpl:
		switch b {
		case dkFuncImpl, dkBindingElt, dkClass, dkEnum, dkParam:
			return true
		case dkFuncOverload:
			return false
		}
	case dkFuncOverload:
		switch b {
		case dkBindingElt, dkClass, dkEnum:
			return true
		}
	case dkClass:
		switch b {
		case dkClass, dkEnum, dkTypeAlias, dkBindingElt, dkParam:
			return true
		}
	case dkEnum:
		// Enum + enum merges in TS when const-ness matches; biome's
		// fixtures accept it unconditionally, so we follow.
		switch b {
		case dkTypeAlias, dkBindingElt, dkParam:
			return true
		}
	case dkInterface:
		switch b {
		case dkTypeAlias:
			return true
		}
	case dkTypeAlias:
		switch b {
		case dkTypeAlias, dkInterface, dkClass, dkEnum:
			return true
		}
	case dkParam:
		switch b {
		case dkVar, dkBindingElt, dkClass, dkEnum, dkFuncImpl:
			return true
		}
	case dkBindingElt:
		switch b {
		case dkBindingElt, dkVar, dkFuncImpl, dkFuncOverload, dkClass, dkEnum, dkParam:
			return true
		}
	}
	return false
}

// --- sloppy-mode hoist pass ----------------------------------------

// hoistPass implements the JS spec's sloppy-mode function-declaration
// hoisting that TS's binder skips. In an ES module or under
// `"use strict"`, a FunctionDeclaration nested in a block stays
// block-scoped; otherwise it hoists to the enclosing function-like
// (or script) scope, where it can collide with sibling vars and other
// hoisted function decls.
func hoistPass(sf *wrapperchecker.Node, report func(*wrapperchecker.Node, string)) {
	isModule := sourceFileIsModule(sf)
	var walkScope func(scope *wrapperchecker.Node, isStrict bool)
	walkScope = func(scope *wrapperchecker.Node, isStrict bool) {
		isStrict = isStrict || prologueHasUseStrict(scope)

		type entry struct {
			vars        []*wrapperchecker.Node
			fnImpls     []*wrapperchecker.Node
			fnOverloads []*wrapperchecker.Node
			params      []*wrapperchecker.Node
		}
		names := map[string]*entry{}
		ensure := func(k string) *entry {
			e := names[k]
			if e == nil {
				e = &entry{}
				names[k] = e
			}
			return e
		}
		var nested []*wrapperchecker.Node

		// Pre-collect parameters of this function-like scope.
		if isFunctionLike(scope) {
			for _, p := range scope.FunctionParameters() {
				pname := p.ParameterName()
				if pname != nil && pname.Kind() == wrapperchecker.KindIdentifier {
					ensure(pname.SourceText()).params = append(ensure(pname.SourceText()).params, pname)
				}
			}
		}

		var walk func(n *wrapperchecker.Node, inBlock bool)
		walk = func(n *wrapperchecker.Node, inBlock bool) {
			if n == nil {
				return
			}
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration:
				name := firstIdentChild(n)
				if name != nil && name.Kind() == wrapperchecker.KindIdentifier {
					if !isStrict || !inBlock {
						e := ensure(name.SourceText())
						if n.FunctionBody() != nil {
							e.fnImpls = append(e.fnImpls, name)
						} else {
							e.fnOverloads = append(e.fnOverloads, name)
						}
					}
				}
				nested = append(nested, n)
				return
			case wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration,
				wrapperchecker.KindConstructor,
				wrapperchecker.KindGetAccessor,
				wrapperchecker.KindSetAccessor,
				wrapperchecker.KindClassStaticBlockDeclaration:
				nested = append(nested, n)
				return
			case wrapperchecker.KindClassDeclaration,
				wrapperchecker.KindClassExpression,
				wrapperchecker.KindEnumDeclaration,
				wrapperchecker.KindInterfaceDeclaration,
				wrapperchecker.KindTypeAliasDeclaration,
				wrapperchecker.KindModuleDeclaration:
				// Type-side declarations don't participate in
				// var/function hoist conflicts; skip their bodies.
				return
			case wrapperchecker.KindVariableDeclaration:
				if isVarDecl(n) {
					name := n.VariableDeclarationName()
					if name != nil && name.Kind() == wrapperchecker.KindIdentifier {
						ensure(name.SourceText()).vars = append(ensure(name.SourceText()).vars, name)
					}
				}
			}
			childInBlock := inBlock
			switch n.Kind() {
			case wrapperchecker.KindBlock,
				wrapperchecker.KindCaseClause,
				wrapperchecker.KindDefaultClause,
				wrapperchecker.KindIfStatement,
				wrapperchecker.KindForStatement,
				wrapperchecker.KindForInStatement,
				wrapperchecker.KindForOfStatement,
				wrapperchecker.KindWhileStatement,
				wrapperchecker.KindDoStatement,
				wrapperchecker.KindLabeledStatement,
				wrapperchecker.KindTryStatement,
				wrapperchecker.KindCatchClause,
				wrapperchecker.KindSwitchStatement:
				childInBlock = true
			}
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c, childInBlock)
				return false
			})
		}
		// Walk the scope's body, not the scope itself (which is the
		// function/static-block/source-file holding the bindings).
		body := scopeBody(scope)
		if body != nil {
			body.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c, false)
				return false
			})
		}

		for name, e := range names {
			totalImpl := len(e.fnImpls)
			totalVar := len(e.vars)
			totalOverload := len(e.fnOverloads)
			totalParam := len(e.params)
			// Var + var: second var.
			if totalVar >= 2 {
				report(e.vars[1], name)
			}
			// Function impls + function impls: second impl.
			if totalImpl >= 2 {
				report(e.fnImpls[1], name)
			}
			// Var + function decl (impl or overload): later one wins.
			if totalVar > 0 && (totalImpl > 0 || totalOverload > 0) {
				later := laterOf(e.vars, e.fnImpls, e.fnOverloads)
				if later != nil {
					report(later, name)
				}
			}
			// Param + var: report the var.
			if totalParam > 0 && totalVar > 0 {
				report(e.vars[0], name)
			}
		}

		for _, n := range nested {
			walkScope(n, isStrict)
		}
	}
	walkScope(sf, isModule)
}

func laterOf(lists ...[]*wrapperchecker.Node) *wrapperchecker.Node {
	var best *wrapperchecker.Node
	for _, list := range lists {
		for _, n := range list {
			if best == nil || n.Pos() > best.Pos() {
				best = n
			}
		}
	}
	return best
}

func scopeBody(scope *wrapperchecker.Node) *wrapperchecker.Node {
	if scope.Kind() == wrapperchecker.KindSourceFile {
		return scope
	}
	if isFunctionLike(scope) {
		return scope.FunctionBody()
	}
	if scope.Kind() == wrapperchecker.KindClassStaticBlockDeclaration {
		// The static block's body is its first Block child.
		var b *wrapperchecker.Node
		scope.ForEachChild(func(c *wrapperchecker.Node) bool {
			if b == nil && c.Kind() == wrapperchecker.KindBlock {
				b = c
				return true
			}
			return false
		})
		return b
	}
	return nil
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		return true
	}
	return false
}

func isVarDecl(n *wrapperchecker.Node) bool {
	if n.IsConstVariableDeclaration() {
		return false
	}
	// Walk up to the VariableDeclarationList and inspect its leading
	// keyword via source text. The wrapper doesn't expose an
	// IsLetDeclaration helper, but the list's first token is the
	// keyword.
	list := n.Parent()
	if list == nil || list.Kind() != wrapperchecker.KindVariableDeclarationList {
		return false
	}
	stmt := list.Parent()
	if stmt == nil {
		return false
	}
	// For-in / for-of / for use the list directly under the statement,
	// not relevant to redeclare hoisting in the same way; ignore.
	if stmt.Kind() == wrapperchecker.KindForInStatement ||
		stmt.Kind() == wrapperchecker.KindForOfStatement {
		return false
	}
	// `using` / `await using` are not vars.
	if list.IsUsingDeclaration() || list.IsAwaitUsingDeclaration() {
		return false
	}
	// The leading keyword in the list's source text identifies the
	// flavor: var | let | const. Strip leading whitespace and look at
	// the first three characters.
	src := list.SourceText()
	for len(src) > 0 && (src[0] == ' ' || src[0] == '\t' || src[0] == '\n' || src[0] == '\r') {
		src = src[1:]
	}
	return len(src) >= 3 && src[:3] == "var"
}

// sourceFileIsModule reports whether the source file is an ES module
// (and therefore implicitly strict).
func sourceFileIsModule(sf *wrapperchecker.Node) bool {
	var isModule bool
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindImportDeclaration,
			wrapperchecker.KindImportEqualsDeclaration,
			wrapperchecker.KindExportDeclaration,
			wrapperchecker.KindExportAssignment,
			wrapperchecker.KindNamespaceExportDeclaration:
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

// prologueHasUseStrict reports whether the function-like body starts
// with a "use strict" directive prologue.
func prologueHasUseStrict(scope *wrapperchecker.Node) bool {
	body := scope
	if isFunctionLike(scope) {
		body = scope.FunctionBody()
		if body == nil {
			return false
		}
	}
	var found bool
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindExpressionStatement {
			return true
		}
		expr := c.ExpressionStatementExpression()
		if expr == nil {
			return true
		}
		if expr.Kind() != wrapperchecker.KindStringLiteral &&
			expr.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
			return true
		}
		text := expr.LiteralText()
		if text == "use strict" {
			found = true
			return true
		}
		return false
	})
	return found
}
