// Package adjacentoverloadsignatures implements the
// adjacent-overload-signatures rule: function and method overload
// signatures that share an identity (name + static-ness + call/construct
// flavor + name-kind) must appear consecutively within a container.
// Once any other member sits between two signatures of the same
// identity, the later occurrence is flagged.
//
// Pure-AST: no type lookups. Containers walked are SourceFile,
// Block, ModuleBlock, ClassDeclaration/Expression,
// InterfaceDeclaration, and TypeLiteral — mirroring oxc.
package adjacentoverloadsignatures

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "adjacent-overload-signatures"

// New constructs an adjacent-overload-signatures rule.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile:           visitContainer,
		wrapperchecker.KindBlock:                visitContainer,
		wrapperchecker.KindModuleBlock:          visitContainer,
		wrapperchecker.KindClassDeclaration:     visitContainer,
		wrapperchecker.KindClassExpression:      visitContainer,
		wrapperchecker.KindInterfaceDeclaration: visitContainer,
		wrapperchecker.KindTypeLiteral:          visitContainer,
	}
}

type methodKind int

const (
	mkNormal methodKind = iota
	mkPrivate
	mkQuoted
	mkExpression
)

type method struct {
	name    string
	static  bool
	callSig bool
	kind    methodKind
	node    *wrapperchecker.Node
}

func sameMethod(a, b *method) bool {
	if a == nil || b == nil {
		return false
	}
	return a.name == b.name && a.static == b.static && a.callSig == b.callSig && a.kind == b.kind
}

func visitContainer(ctx *engine.Context, n *wrapperchecker.Node) {
	var methods []*method
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		methods = append(methods, getMethod(c))
		return false
	})
	var last *method
	var seen []*method
	for _, m := range methods {
		if m == nil {
			last = nil
			continue
		}
		seenSame := false
		for _, s := range seen {
			if sameMethod(m, s) {
				seenSame = true
				break
			}
		}
		if seenSame && !sameMethod(m, last) {
			display := m.name
			if m.static {
				display = "static " + display
			}
			ctx.Report(m.node, fmt.Sprintf("All %q signatures should be adjacent.", display))
		} else {
			seen = append(seen, m)
		}
		last = m
	}
}

func getMethod(c *wrapperchecker.Node) *method {
	if c == nil {
		return nil
	}
	switch c.Kind() {
	case wrapperchecker.KindConstructor:
		return &method{name: "constructor", kind: mkNormal, node: c}
	case wrapperchecker.KindCallSignature:
		return &method{name: "call", callSig: true, kind: mkNormal, node: c}
	case wrapperchecker.KindConstructSignature:
		return &method{name: "new", kind: mkNormal, node: c}
	case wrapperchecker.KindMethodDeclaration, wrapperchecker.KindMethodSignature:
		return methodFromKeyed(c)
	case wrapperchecker.KindFunctionDeclaration:
		name := c.DeclarationName()
		if name == nil || name.Kind() != wrapperchecker.KindIdentifier {
			return nil
		}
		return &method{name: name.LiteralText(), kind: mkNormal, node: c}
	}
	return nil
}

func methodFromKeyed(c *wrapperchecker.Node) *method {
	name := c.DeclarationName()
	if name == nil {
		return nil
	}
	// Unwrap `['foo']` / `[42]` style computed keys: oxc treats a
	// computed key whose inner expression is a literal as if it had
	// been written without brackets, so `['foo']` collides with `foo`.
	// Truly dynamic keys (e.g. `[Symbol.iterator]`) keep mkExpression.
	keyNode := name
	if name.Kind() == wrapperchecker.KindComputedPropertyName {
		inner := name.FirstChild()
		if inner == nil {
			return &method{kind: mkExpression, static: c.HasStaticModifier(), node: c}
		}
		keyNode = inner
	}
	static := c.HasStaticModifier()
	switch keyNode.Kind() {
	case wrapperchecker.KindIdentifier, wrapperchecker.KindStringLiteral:
		return &method{name: keyNode.LiteralText(), kind: mkNormal, static: static, node: c}
	case wrapperchecker.KindPrivateIdentifier:
		return &method{name: keyNode.LiteralText(), kind: mkPrivate, static: static, node: c}
	case wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindNullKeyword:
		return &method{name: keyNode.LiteralText(), kind: mkQuoted, static: static, node: c}
	}
	return &method{kind: mkExpression, static: static, node: c}
}
