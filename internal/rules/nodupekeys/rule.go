// Package nodupekeys implements the no-dupe-keys rule: flag duplicate
// keys in object literals. JavaScript silently keeps only the last
// value assigned to each key, so the earlier assignments produce
// either dead code (an init overwritten by a later init) or a runtime
// bug (a value the author intended to keep, silently dropped).
//
// Each object literal is analyzed in isolation. Keys are extracted
// from the four common property forms:
//
//   - PropertyAssignment ({ a: 1 })
//   - ShorthandPropertyAssignment ({ a })
//   - MethodDeclaration ({ a() {} })
//   - GetAccessor / SetAccessor ({ get a() {}, set a(v) {} })
//
// Getter/setter pairs with the same name are legal and not flagged.
// Anything else with a repeated name is. Computed keys (`[expr]: 1`)
// are intentionally skipped: their runtime value cannot be statically
// determined and the rule refuses to guess. Spread elements (`...x`)
// carry no static key and are also skipped.
//
// Key text comes from [Node.LiteralText] on the name identifier,
// string literal, or numeric literal. This canonicalizes `{ 1:..., "1":...}`
// to the same key — matching JavaScript's own coercion — without
// needing any runtime evaluation.
package nodupekeys

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-dupe-keys"

// New constructs a nodupekeys rule instance ready for registration with
// the engine.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindObjectLiteralExpression: visit,
	}
}

// propertyKind labels what a property does with its name slot.
// "init" covers regular properties, shorthand, and methods — all of
// which collide with one another. "get" and "set" only coexist as a
// matched pair.
type propertyKind string

const (
	kindInit propertyKind = "init"
	kindGet  propertyKind = "get"
	kindSet  propertyKind = "set"
)

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Per-name, the set of property kinds we've already accepted. A
	// second appearance is fine only if it completes the {get, set}
	// pair; everything else triggers a diagnostic.
	seen := map[string]map[propertyKind]bool{}
	for _, prop := range n.ObjectProperties() {
		name, kind, ok := propertyKey(prop)
		if !ok {
			continue
		}
		existing := seen[name]
		if existing == nil {
			seen[name] = map[propertyKind]bool{kind: true}
			continue
		}
		if completesAccessorPair(existing, kind) {
			existing[kind] = true
			continue
		}
		ctx.Report(prop, "duplicate key '"+name+"'")
		// Track the kind so a third occurrence of the same name still
		// only reports once per extra entry rather than accumulating.
		existing[kind] = true
	}
}

// completesAccessorPair reports whether adding kind to existing turns
// a lone {get} or {set} into the legal {get, set} pair.
func completesAccessorPair(existing map[propertyKind]bool, kind propertyKind) bool {
	if len(existing) != 1 {
		return false
	}
	if existing[kindGet] && kind == kindSet {
		return true
	}
	if existing[kindSet] && kind == kindGet {
		return true
	}
	return false
}

// propertyKey extracts the static key name and the kind of property
// for a node nested under an ObjectLiteralExpression. Returns ok=false
// for spread elements, computed keys, and anything else without a
// resolvable static key.
func propertyKey(prop *wrapperchecker.Node) (name string, kind propertyKind, ok bool) {
	switch prop.Kind() {
	case wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment,
		wrapperchecker.KindMethodDeclaration:
		kind = kindInit
	case wrapperchecker.KindGetAccessor:
		kind = kindGet
	case wrapperchecker.KindSetAccessor:
		kind = kindSet
	default:
		// Spread, or some other property form we don't analyze.
		return "", "", false
	}
	id := propertyNameNode(prop)
	if id == nil {
		return "", "", false
	}
	text := id.LiteralText()
	if text == "" {
		return "", "", false
	}
	return text, kind, true
}

// propertyNameNode returns the identifier, string-literal, or
// numeric-literal node that names the property, or nil for computed
// names. Skipping ComputedPropertyName here is what makes the rule
// silent on `{ [expr]: 1 }`.
func propertyNameNode(prop *wrapperchecker.Node) *wrapperchecker.Node {
	var name *wrapperchecker.Node
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		if name != nil {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPrivateIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNumericLiteral:
			name = c
			return true
		}
		return false
	})
	return name
}
