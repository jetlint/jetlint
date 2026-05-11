// Package nomixedenums implements the no-mixed-enums rule: flag
// enum declarations that combine string and number members.
package nomixedenums

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-mixed-enums"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindEnumDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	first := siblingDeclarationsKind(ctx, n)
	reported := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if reported {
			return false
		}
		if c.Kind() != wrapperchecker.KindEnumMember {
			return false
		}
		kind := memberKind(ctx, c)
		if kind == "" {
			return false
		}
		if first == "" {
			first = kind
			return false
		}
		if kind != first {
			ctx.Report(c, "enum mixes "+first+" and "+kind+" member values")
			reported = true
		}
		return false
	})
}

// siblingDeclarationsKind looks up the enum's symbol and inspects its
// other declarations (TS allows enum merging). If a sibling declaration
// already established a kind, the current declaration must match it.
// Returns "" when no sibling kind can be determined.
func siblingDeclarationsKind(ctx *engine.Context, n *wrapperchecker.Node) string {
	name := enumName(n)
	if name == nil {
		return ""
	}
	sym := ctx.Checker().SymbolOf(name)
	if sym == nil {
		return ""
	}
	for _, decl := range sym.Declarations() {
		if decl.Kind() != wrapperchecker.KindEnumDeclaration {
			continue
		}
		// Only consider declarations that came BEFORE this one; we're
		// flagging the later declaration that diverges, not the first.
		if decl.Pos() >= n.Pos() {
			continue
		}
		if k := firstMemberKind(ctx, decl); k != "" {
			return k
		}
	}
	return ""
}

func enumName(n *wrapperchecker.Node) *wrapperchecker.Node {
	var name *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == nil {
			name = c
			return true
		}
		return false
	})
	return name
}

func firstMemberKind(ctx *engine.Context, decl *wrapperchecker.Node) string {
	var kind string
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindEnumMember {
			return false
		}
		if k := memberKind(ctx, c); k != "" {
			kind = k
			return true
		}
		return false
	})
	return kind
}

// memberKind returns "string", "number", or "" for an enum member.
// Members without an initializer take the auto-numbering path
// (number). String members yield "string"; number members yield
// "number"; everything else (booleans, function calls returning
// neither) yields "".
func memberKind(ctx *engine.Context, m *wrapperchecker.Node) string {
	init := m.EnumMemberInitializer()
	if init == nil {
		return "number"
	}
	t := ctx.TypeOf(init)
	if t == nil {
		return ""
	}
	if t.IsStringLike() {
		return "string"
	}
	if t.IsNumberLike() {
		return "number"
	}
	return ""
}
