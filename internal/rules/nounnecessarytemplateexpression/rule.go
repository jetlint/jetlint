// Package nounnecessarytemplateexpression implements the
// no-unnecessary-template-expression rule: flag template-literal
// interpolations of literal-typed expressions and trivial
// single-interpolation templates that just stringify an already-string
// value.
package nounnecessarytemplateexpression

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unnecessary-template-expression"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTemplateExpression:  visitTemplate,
		wrapperchecker.KindTemplateLiteralType: visitTemplateLiteralType,
	}
}

type span struct {
	expr *wrapperchecker.Node // expression or type-node being interpolated
	tail *wrapperchecker.Node // TemplateMiddle or TemplateTail closing the span
}

func splitTemplate(n *wrapperchecker.Node) (head *wrapperchecker.Node, spans []span) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindTemplateHead:
			head = c
		case wrapperchecker.KindTemplateSpan, wrapperchecker.KindTemplateLiteralTypeSpan:
			s := span{}
			c.ForEachChild(func(inner *wrapperchecker.Node) bool {
				switch inner.Kind() {
				case wrapperchecker.KindTemplateMiddle, wrapperchecker.KindTemplateTail:
					s.tail = inner
				default:
					if s.expr == nil {
						s.expr = inner
					}
				}
				return false
			})
			spans = append(spans, s)
		}
		return false
	})
	return
}

func visitTemplate(ctx *engine.Context, n *wrapperchecker.Node) {
	if parent := n.Parent(); parent != nil && parent.Kind() == wrapperchecker.KindTaggedTemplateExpression {
		return
	}
	head, spans := splitTemplate(n)
	if head == nil || len(spans) == 0 {
		return
	}
	if len(spans) == 1 && head.LiteralText() == "" && spans[0].tail.LiteralText() == "" && !hasInterpComment(spans[0]) {
		if t := ctx.TypeOf(spans[0].expr); t != nil && isUnderlyingTypeString(t) && !isEnumMemberType(t) {
			ctx.Report(spans[0].expr, "unnecessary template-literal wrapping a string-typed expression")
			return
		}
	}
	for _, s := range spans {
		if hasInterpComment(s) {
			continue
		}
		if !isUnnecessaryValueInterp(s.expr) {
			continue
		}
		if hasTrailingNewlineQuasi(s.tail) && exprIsWhitespaceLiteral(s.expr) {
			continue
		}
		ctx.Report(s.expr, "unnecessary template-literal interpolation of a literal value")
	}
}

func visitTemplateLiteralType(ctx *engine.Context, n *wrapperchecker.Node) {
	head, spans := splitTemplate(n)
	if head == nil || len(spans) == 0 {
		return
	}
	if len(spans) == 1 && head.LiteralText() == "" && spans[0].tail.LiteralText() == "" && !hasInterpComment(spans[0]) {
		if t := ctx.Checker().TypeFromTypeNode(spans[0].expr); t != nil && !t.IsTypeParameter() && isUnderlyingTypeString(t) && !isEnumMemberType(t) {
			ctx.Report(spans[0].expr, "unnecessary template-literal type wrapping a string-typed type")
			return
		}
	}
	for _, s := range spans {
		if hasInterpComment(s) {
			continue
		}
		if !isUnnecessaryTypeInterp(s.expr) {
			continue
		}
		if hasTrailingNewlineQuasi(s.tail) && typeIsWhitespaceLiteral(s.expr) {
			continue
		}
		ctx.Report(s.expr, "unnecessary template-literal-type interpolation of a literal type")
	}
}

// exprIsWhitespaceLiteral reports whether expr is a string-literal or
// no-substitution-template whose value is purely whitespace.
func exprIsWhitespaceLiteral(expr *wrapperchecker.Node) bool {
	if expr == nil {
		return false
	}
	switch expr.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return isWhitespaceString(expr.LiteralText())
	case wrapperchecker.KindTemplateExpression:
		// `${`literal`}` — the inner template has no interpolations
		// when its only child is a NoSubstitutionTemplateLiteral.
		return false
	}
	return false
}

// typeIsWhitespaceLiteral mirrors exprIsWhitespaceLiteral for type
// nodes — only LiteralType wrapping a whitespace string qualifies.
func typeIsWhitespaceLiteral(t *wrapperchecker.Node) bool {
	if t == nil || t.Kind() != wrapperchecker.KindLiteralType {
		return false
	}
	inner := t.FirstChild()
	if inner == nil {
		return false
	}
	switch inner.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return isWhitespaceString(inner.LiteralText())
	}
	return false
}

// hasInterpComment reports whether any comment lies inside a template
// span — between the opening `${` and the expression, or between the
// expression and the closing `}`. typescript-eslint preserves comments
// and skips diagnostic reporting whenever a span carries one.
func hasInterpComment(s span) bool {
	if s.expr != nil {
		t := s.expr.LeadingTriviaText()
		if strings.Contains(t, "/*") || strings.Contains(t, "//") {
			return true
		}
	}
	if s.tail != nil {
		t := s.tail.LeadingTriviaText()
		if strings.Contains(t, "/*") || strings.Contains(t, "//") {
			return true
		}
	}
	return false
}

// hasTrailingWhitespaceLine reports whether the next quasi's text
// begins with a newline. Combined with a whitespace-only interpolation,
// upstream considers this a deliberate trailing-whitespace marker and
// exempts the span.
func hasTrailingNewlineQuasi(tail *wrapperchecker.Node) bool {
	if tail == nil {
		return false
	}
	t := tail.LiteralText()
	return strings.HasPrefix(t, "\n") || strings.HasPrefix(t, "\r\n")
}

func isWhitespaceString(s string) bool {
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			continue
		}
		return false
	}
	return true
}

func isUnnecessaryValueInterp(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindTemplateExpression:
		return true
	}
	if n.Kind() == wrapperchecker.KindIdentifier {
		switch n.LiteralText() {
		case "undefined", "Infinity", "NaN":
			return true
		}
	}
	return false
}

func isUnnecessaryTypeInterp(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindLiteralType,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindUndefinedKeyword,
		wrapperchecker.KindTemplateLiteralType:
		// Empty string literals are exempt — `${''}` is a deliberate
		// whitespace marker upstream considers load-bearing.
		return !isEmptyStringLiteralType(n)
	}
	return false
}

func isEmptyStringLiteralType(t *wrapperchecker.Node) bool {
	if t == nil || t.Kind() != wrapperchecker.KindLiteralType {
		return false
	}
	inner := t.FirstChild()
	if inner == nil {
		return false
	}
	switch inner.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return inner.LiteralText() == ""
	}
	return false
}

// isUnderlyingTypeString matches typescript-eslint's helper of the
// same name: a union is string-like when every member is, an
// intersection is string-like when any member is, and primitives are
// checked directly. Walks type-parameter constraints so `T extends
// string` counts.
func isUnderlyingTypeString(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isUnderlyingTypeString(m) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isUnderlyingTypeString(m) {
				return true
			}
		}
		return false
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isUnderlyingTypeString(c)
		}
		return false
	}
	return t.IsStringLike()
}

// isEnumMemberType reports whether t (or any constituent of t) is an
// enum member. Enum members format differently from their underlying
// string and substituting the literal value loses the enum reference.
func isEnumMemberType(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isEnumMemberType(m) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isEnumMemberType(m) {
				return true
			}
		}
		return false
	}
	return t.IsEnumLike()
}

