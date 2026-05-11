// Package nounnecessaryqualifier implements the
// no-unnecessary-qualifier rule: flag namespace prefixes inside the
// same namespace.
package nounnecessaryqualifier

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unnecessary-qualifier"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindQualifiedName:            visitQualified,
		wrapperchecker.KindPropertyAccessExpression: visitPropertyAccess,
	}
}

func visitQualified(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n, qualifiedLeft(n), qualifiedRight(n))
}

func visitPropertyAccess(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n, n.PropertyAccessReceiver(), n.PropertyAccessName())
}

func check(ctx *engine.Context, n, prefix *wrapperchecker.Node, propertyName string) {
	if prefix == nil {
		return
	}
	if prefix.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	prefixName := prefix.LiteralText()
	if prefixName == "" || propertyName == "" {
		return
	}
	if enclosedByNamespace(n, prefixName) {
		// If a closer-scoped declaration shadows the property name, the
		// qualifier is needed to disambiguate. Walk from n outward and
		// stop once we hit the declaring namespace.
		if shadowedBeforeNamespace(n, prefixName, propertyName) {
			return
		}
		ctx.Report(n, "qualifier is unnecessary inside the enclosing namespace")
		return
	}
	if enclosedByMatchingModuleAugmentation(ctx, n, prefix) {
		ctx.Report(n, "qualifier is unnecessary inside the augmented module")
	}
}

// enclosedByMatchingModuleAugmentation reports whether `n` sits inside
// a `declare module 'X' { ... }` block whose target module is the same
// thing the `prefix` identifier names. Used to catch the augmentation
// pattern:
//
//	import * as Foo from './foo';
//	declare module './foo' {
//	  const x: Foo.T = 3;
//	}
//
// where `Foo.` is redundant because the body is already in `./foo`'s
// scope. Symbol equality after alias-walking is the load-bearing check —
// the prefix's alias resolves to the module symbol, whose declarations
// include the enclosing augmentation block.
func enclosedByMatchingModuleAugmentation(ctx *engine.Context, n, prefix *wrapperchecker.Node) bool {
	mod := enclosingStringNamedModule(n)
	if mod == nil {
		return false
	}
	modSym := ctx.Checker().SymbolOf(mod)
	if modSym == nil {
		return false
	}
	prefixSym := ctx.Checker().SymbolOf(prefix)
	if prefixSym == nil {
		return false
	}
	if prefixSym.Same(modSym) {
		return true
	}
	for _, d := range prefixSym.AllDeclarationsFollowingAliases() {
		if d == nil {
			continue
		}
		ds := ctx.Checker().SymbolOf(d)
		if ds != nil && ds.Same(modSym) {
			return true
		}
	}
	return false
}

// enclosingStringNamedModule returns the name node of the closest
// ancestor ModuleDeclaration whose name is a string literal (the
// `declare module 'X'` augmentation form). Nil when not inside one.
func enclosingStringNamedModule(n *wrapperchecker.Node) *wrapperchecker.Node {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		if cur.Kind() != wrapperchecker.KindModuleDeclaration {
			continue
		}
		var nameNode *wrapperchecker.Node
		cur.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindStringLiteral, wrapperchecker.KindIdentifier:
				nameNode = c
				return true
			}
			return false
		})
		if nameNode != nil && nameNode.Kind() == wrapperchecker.KindStringLiteral {
			return nameNode
		}
	}
	return nil
}

// qualifiedRight returns the right-hand identifier of a QualifiedName,
// e.g. `T` in `X.T`.
func qualifiedRight(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
		}
		return false
	})
	return name
}

// shadowedBeforeNamespace reports whether any enclosing block strictly
// inside the namespace named `prefixName` declares a value named
// `propertyName`, OR whether the named namespace itself does not
// declare `propertyName` (in which case the qualifier resolves to
// some merged declaration such as an enum or imported namespace and
// is load-bearing). The namespace's own body otherwise is excluded —
// its declarations are the symbols that `prefixName.propertyName`
// references, not shadows of them.
func shadowedBeforeNamespace(n *wrapperchecker.Node, prefixName, propertyName string) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		// If we reach the namespace's body or the namespace itself,
		// confirm the namespace actually owns `propertyName` before
		// declaring the qualifier unnecessary.
		if cur.Kind() == wrapperchecker.KindModuleDeclaration && moduleName(cur) == prefixName {
			return !namespaceDeclares(cur, propertyName)
		}
		if cur.Kind() == wrapperchecker.KindModuleBlock {
			parent := cur.Parent()
			if parent != nil && parent.Kind() == wrapperchecker.KindModuleDeclaration && moduleName(parent) == prefixName {
				return !namespaceDeclares(parent, propertyName)
			}
		}
		if blockDeclares(cur, propertyName) {
			return true
		}
	}
	return false
}

// namespaceDeclares reports whether the given module/namespace
// declaration declares `name` directly inside its body. Walks the
// ModuleBlock's immediate statements; nested namespaces, enums,
// classes, type aliases, and var/let/const declarations all count.
func namespaceDeclares(mod *wrapperchecker.Node, name string) bool {
	found := false
	mod.ForEachChild(func(body *wrapperchecker.Node) bool {
		if body.Kind() != wrapperchecker.KindModuleBlock {
			return false
		}
		body.ForEachChild(func(stmt *wrapperchecker.Node) bool {
			if declaresName(stmt, name) {
				found = true
				return true
			}
			return false
		})
		return true
	})
	return found
}

// blockDeclares reports whether the immediate children of `block` (or
// its body) include a type/var/function/enum/namespace declaration of
// the given simple name.
func blockDeclares(block *wrapperchecker.Node, name string) bool {
	found := false
	visit := func(c *wrapperchecker.Node) bool {
		if declaresName(c, name) {
			found = true
			return true
		}
		return false
	}
	block.ForEachChild(func(c *wrapperchecker.Node) bool {
		if found {
			return true
		}
		// ModuleDeclaration's body is the ModuleBlock — descend one
		// level to see siblings of the access expression.
		if c.Kind() == wrapperchecker.KindModuleBlock || c.Kind() == wrapperchecker.KindBlock {
			c.ForEachChild(visit)
			return found
		}
		return visit(c)
	})
	return found
}

// declaresName reports whether stmt declares a top-level binding
// named `name`. Covers type aliases, var/let/const declarations,
// function/class/enum declarations, and nested namespaces.
func declaresName(stmt *wrapperchecker.Node, name string) bool {
	switch stmt.Kind() {
	case wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindModuleDeclaration:
		return moduleName(stmt) == name
	case wrapperchecker.KindVariableStatement:
		hit := false
		stmt.ForEachChild(func(decllist *wrapperchecker.Node) bool {
			decllist.ForEachChild(func(decl *wrapperchecker.Node) bool {
				if decl.Kind() != wrapperchecker.KindVariableDeclaration {
					return false
				}
				if moduleName(decl) == name {
					hit = true
					return true
				}
				return false
			})
			return hit
		})
		return hit
	}
	return false
}

// enclosedByNamespace reports whether n sits inside a namespace or
// enum named `name`. Enums introduce a `Foo.Bar` qualified scope just
// like a namespace does.
func enclosedByNamespace(n *wrapperchecker.Node, name string) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindModuleDeclaration, wrapperchecker.KindEnumDeclaration:
			if moduleName(cur) == name {
				return true
			}
		}
	}
	return false
}

func qualifiedLeft(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		first = c
		return true
	})
	return first
}

func moduleName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
