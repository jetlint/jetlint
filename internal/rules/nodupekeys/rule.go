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
	"strings"

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

// propertyKind labels what a property does with its name slot. All
// "init"-family kinds collide with each other; get/set coexist as a
// matched pair. "proto" is the special `__proto__: value` form which
// sets the prototype rather than creating a property — two protos
// are a syntax error, but a proto plus a shorthand/method/accessor
// `__proto__` is legal (the latter creates a property).
type propertyKind string

const (
	kindInit      propertyKind = "init"
	kindShorthand propertyKind = "shorthand"
	kindMethod    propertyKind = "method"
	kindGet       propertyKind = "get"
	kindSet       propertyKind = "set"
	kindProto     propertyKind = "proto"
)


func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Per-name, the set of property kinds we've already accepted.
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
		if !collides(existing, kind) {
			existing[kind] = true
			continue
		}
		ctx.Report(prop, "duplicate key '"+name+"'")
		existing[kind] = true
	}
}

// collides reports whether kind clashes with at least one of the
// kinds already seen at the same name. get+set are a legal pair; the
// __proto__ init form is only a real duplicate against another
// __proto__ init form.
func collides(existing map[propertyKind]bool, kind propertyKind) bool {
	for prev := range existing {
		if collisionPair(prev, kind) {
			return true
		}
	}
	return false
}

func collisionPair(a, b propertyKind) bool {
	if a == kindProto || b == kindProto {
		// __proto__ as init only collides with another __proto__ init.
		// Combinations with shorthand/method/accessor create a normal
		// `__proto__` property alongside the prototype assignment.
		return a == kindProto && b == kindProto
	}
	if a == kindGet && b == kindSet {
		return false
	}
	if a == kindSet && b == kindGet {
		return false
	}
	return true
}

// propertyKey extracts the static key name and the kind of property
// for a node nested under an ObjectLiteralExpression. Returns
// ok=false for spread elements and computed keys whose value isn't
// statically known.
func propertyKey(prop *wrapperchecker.Node) (name string, kind propertyKind, ok bool) {
	switch prop.Kind() {
	case wrapperchecker.KindPropertyAssignment:
		kind = kindInit
	case wrapperchecker.KindShorthandPropertyAssignment:
		kind = kindShorthand
	case wrapperchecker.KindMethodDeclaration:
		kind = kindMethod
	case wrapperchecker.KindGetAccessor:
		kind = kindGet
	case wrapperchecker.KindSetAccessor:
		kind = kindSet
	default:
		return "", "", false
	}
	id := propertyNameNode(prop)
	if id == nil {
		return "", "", false
	}
	computed := isComputedName(prop)
	text, gotName := nameValue(id, computed)
	if !gotName {
		return "", "", false
	}
	// The __proto__: value init form sets the prototype rather than
	// creating an own property. Only this exact shape gets the proto
	// kind; shorthand/method/accessor `__proto__` create normal
	// properties and stay as their own kinds.
	if text == "__proto__" && kind == kindInit && !computed {
		kind = kindProto
	}
	return text, kind, true
}

// nameValue returns the static name represented by a property-name
// node. For non-computed keys, the identifier text IS the name. For
// computed keys, only true literals (strings, numbers, templates with
// no substitutions, regexes, bigints) resolve statically — an
// identifier like `[a]` looks up a runtime value and is treated as
// non-static.
func nameValue(n *wrapperchecker.Node, computed bool) (string, bool) {
	switch n.Kind() {
	case wrapperchecker.KindIdentifier, wrapperchecker.KindPrivateIdentifier:
		if computed {
			return "", false
		}
		return n.LiteralText(), true
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText(), true
	case wrapperchecker.KindNumericLiteral:
		return n.LiteralText(), true
	case wrapperchecker.KindBigIntLiteral:
		// `1n` and `1` coerce to the same property key "1" per ToString.
		return strings.TrimSuffix(n.LiteralText(), "n"), true
	case wrapperchecker.KindRegularExpressionLiteral:
		// `[/re/]: 1` and `'/re/': 1` collide because computed-key
		// regex stringifies to its source text.
		return n.LiteralText(), true
	}
	return "", false
}

// isComputedName reports whether prop uses `[expr]` for its key. Only
// matters for distinguishing `__proto__: x` from `['__proto__']: x` —
// the latter sets a regular property, not the prototype.
func isComputedName(prop *wrapperchecker.Node) bool {
	first := prop.FirstChild()
	if first == nil {
		return false
	}
	return !isStaticNameKind(first.Kind())
}

// isStaticNameKind reports whether the given kind is a leaf name
// node (identifier or literal). Anything else in the name slot of an
// ObjectLiteral property is a ComputedPropertyName wrapper.
func isStaticNameKind(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindPrivateIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return true
	}
	return false
}

// propertyNameNode returns the node carrying the property's key. For
// computed keys (`[expr]: value`) the first child is a
// ComputedPropertyName wrapper; we descend into its single child so
// nameValue can decide whether the expression resolves to a
// constant.
func propertyNameNode(prop *wrapperchecker.Node) *wrapperchecker.Node {
	first := prop.FirstChild()
	if first == nil {
		return nil
	}
	if isStaticNameKind(first.Kind()) {
		return first
	}
	return first.FirstChild()
}
