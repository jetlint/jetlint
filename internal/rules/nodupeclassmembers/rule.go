// Package nodupeclassmembers implements the no-dupe-class-members
// rule: declaring two non-overload members of the same class with the
// same static/private/key triple silently overwrites the first
// declaration, almost always a copy-paste mistake. Getter+setter pairs
// of the same name are exempt.
package nodupeclassmembers

import (
	"fmt"
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-dupe-class-members"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: r.visit,
		wrapperchecker.KindClassExpression:  r.visit,
	}
}

type memberKey struct {
	name    string
	static  bool
	private bool
}

type memberInfo struct {
	node *wrapperchecker.Node
	kind memberKind
}

type memberKind int

const (
	kindMethod memberKind = iota
	kindGetter
	kindSetter
	kindProperty
	kindAccessor
)

func (r *rule) visit(ctx *engine.Context, class *wrapperchecker.Node) {
	seen := map[memberKey]memberInfo{}
	class.ForEachChild(func(member *wrapperchecker.Node) bool {
		info, ok := classify(member)
		if !ok {
			return false
		}
		isStatic := member.HasStaticModifier()
		// TS-Go parses both `constructor()` and `static constructor()`
		// as KindConstructor, but only the non-static form is the actual
		// class constructor. The static variant is a regular method named
		// "constructor" and should be tracked for duplicate detection.
		var nameNode *wrapperchecker.Node
		var name string
		if member.Kind() == wrapperchecker.KindConstructor {
			if !isStatic {
				return false
			}
			name = "constructor"
		} else {
			nameNode = member.DeclarationName()
			if nameNode == nil {
				return false
			}
			var ok bool
			name, ok = staticName(nameNode)
			if !ok {
				return false
			}
		}
		// JS spec: a non-static method whose key is an Identifier or
		// StringLiteral with the name `constructor` declares the class
		// constructor and is not a normal member. TS surfaces only the
		// Identifier form as KindConstructor; we have to recognise the
		// StringLiteral form ourselves so cases like
		// `'constructor'() {} [`constructor`]() {}` do not falsely
		// report — the first is the constructor, only the second counts.
		if !isStatic && info == kindMethod && name == "constructor" {
			if k := nameNode.Kind(); k == wrapperchecker.KindIdentifier || k == wrapperchecker.KindStringLiteral {
				return false
			}
		}
		key := memberKey{name: name, static: isStatic, private: isPrivate(nameNode)}
		prev, exists := seen[key]
		if !exists {
			seen[key] = memberInfo{node: member, kind: info}
			return false
		}
		if isAccessorPair(prev.kind, info) {
			// Replace so a later collision with the same kind reports.
			seen[key] = memberInfo{node: member, kind: info}
			return false
		}
		ctx.Report(member, fmt.Sprintf("Duplicate class member %q.", name))
		return false
	})
}

// classify returns the member kind and whether the member counts.
// Overload signatures (MethodDeclaration without a body) and the
// class's own Constructor are skipped.
func classify(member *wrapperchecker.Node) (memberKind, bool) {
	switch member.Kind() {
	case wrapperchecker.KindMethodDeclaration:
		if !hasFunctionBody(member) {
			return 0, false
		}
		return kindMethod, true
	case wrapperchecker.KindConstructor:
		return kindMethod, true
	case wrapperchecker.KindGetAccessor:
		return kindGetter, true
	case wrapperchecker.KindSetAccessor:
		return kindSetter, true
	case wrapperchecker.KindPropertyDeclaration:
		return kindProperty, true
	}
	return 0, false
}

func hasFunctionBody(method *wrapperchecker.Node) bool {
	body := method.FunctionBody()
	return body != nil
}

func isPrivate(nameNode *wrapperchecker.Node) bool {
	if nameNode == nil {
		return false
	}
	return nameNode.Kind() == wrapperchecker.KindPrivateIdentifier
}

// staticName returns the statically-known key text of a class member's
// name node, or false when the key cannot be resolved at compile time
// (e.g. `[foo]`, `[a + b]`, a template literal with substitutions).
func staticName(name *wrapperchecker.Node) (string, bool) {
	if name == nil {
		return "", false
	}
	switch name.Kind() {
	case wrapperchecker.KindIdentifier, wrapperchecker.KindPrivateIdentifier:
		return name.LiteralText(), true
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return name.LiteralText(), true
	case wrapperchecker.KindNumericLiteral:
		return normalizeNumeric(name.LiteralText()), true
	case wrapperchecker.KindBigIntLiteral:
		return normalizeBigInt(name.LiteralText()), true
	case wrapperchecker.KindComputedPropertyName:
		inner := name.FirstChild()
		// Identifier inside `[…]` is a variable reference, not a static key.
		if inner != nil && inner.Kind() == wrapperchecker.KindIdentifier {
			return "", false
		}
		return staticName(inner)
	case wrapperchecker.KindNullKeyword:
		return "null", true
	}
	return "", false
}

// normalizeNumeric converts a numeric literal source text to its JS
// number-to-string form so `10`, `1e1`, `0x0a` all compare equal.
func normalizeNumeric(text string) string {
	clean := strings.ReplaceAll(text, "_", "")
	v, err := strconv.ParseFloat(clean, 64)
	if err == nil {
		return jsNumberToString(v)
	}
	// Hex / octal / binary literals don't parse as float — fall back to
	// integer parsing on the prefixed forms.
	if len(clean) > 2 {
		var base int
		switch strings.ToLower(clean[:2]) {
		case "0x":
			base = 16
		case "0o":
			base = 8
		case "0b":
			base = 2
		}
		if base != 0 {
			if i, err := strconv.ParseInt(clean[2:], base, 64); err == nil {
				return strconv.FormatInt(i, 10)
			}
		}
	}
	return clean
}

func normalizeBigInt(text string) string {
	if strings.HasSuffix(text, "n") {
		text = text[:len(text)-1]
	}
	return normalizeNumeric(text)
}

// jsNumberToString approximates JS's Number.prototype.toString without
// the radix argument: integers stringify without a trailing `.0`, and
// the shortest representation that round-trips is preferred.
func jsNumberToString(v float64) string {
	if v == float64(int64(v)) && v >= -1e15 && v <= 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// isAccessorPair reports whether the two members are a getter/setter
// pair of the same name — the one case where same-key duplicates are
// intentional and not reported.
func isAccessorPair(a, b memberKind) bool {
	return (a == kindGetter && b == kindSetter) || (a == kindSetter && b == kindGetter)
}
