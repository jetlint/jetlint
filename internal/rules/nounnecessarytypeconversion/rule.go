// Package nounnecessarytypeconversion implements the
// no-unnecessary-type-conversion rule: flag `String(s)`, `Number(n)`,
// `Boolean(b)`, `BigInt(b)`, `s.toString()`, `''+s`, `s += ''`, `+n`,
// `~~n`, `!!b` where the argument is already the target primitive type.
package nounnecessarytypeconversion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-type-conversion"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:        visitCall,
		wrapperchecker.KindBinaryExpression:      visitBinary,
		wrapperchecker.KindPrefixUnaryExpression: visitUnary,
	}
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	args := n.CallArguments()

	// String(x), Number(x), Boolean(x), BigInt(x).
	if callee.Kind() == wrapperchecker.KindIdentifier && len(args) == 1 {
		name := callee.LiteralText()
		if isPrimitiveCtorName(name) && !n.FileHasTopLevelDeclaration(name) {
			at := ctx.TypeOf(args[0])
			if at != nil && checkPrimitive(at, name) {
				ctx.Report(n, "redundant "+name+"() — value is already "+primName(name)+"-typed")
			}
		}
		return
	}

	// x.toString() where x is string-like.
	if callee.Kind() == wrapperchecker.KindPropertyAccessExpression && len(args) == 0 {
		propName := propertyAccessName(callee)
		if propName != "toString" {
			return
		}
		recv := callee.FirstChild()
		if recv == nil {
			return
		}
		t := ctx.TypeOf(recv)
		if t == nil {
			return
		}
		// Enum members carry the enum's name in TypeScript even when
		// their literal value is a string; .toString() is meaningful
		// for converting from the enum to a plain string.
		if t.IsEnumLike() {
			return
		}
		if isLikeWithConstraint(t, "string") {
			ctx.Report(n, "redundant .toString() — value is already string-typed")
		}
	}
}

func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	switch op {
	case wrapperchecker.KindPlusToken:
		// `"" + x` or `x + ""` where x is already string.
		if isEmptyStringLiteral(left) {
			if t := ctx.TypeOf(right); t != nil && isLikeWithConstraint(t, "string") {
				ctx.Report(n, "redundant '' + x — value is already string-typed")
			}
			return
		}
		if isEmptyStringLiteral(right) {
			if t := ctx.TypeOf(left); t != nil && isLikeWithConstraint(t, "string") {
				ctx.Report(n, "redundant x + '' — value is already string-typed")
			}
		}
	case wrapperchecker.KindPlusEqualsToken:
		// `x += ""` where x is already string.
		if isEmptyStringLiteral(right) {
			if t := ctx.TypeOf(left); t != nil && isLikeWithConstraint(t, "string") {
				ctx.Report(n, "redundant x += '' — value is already string-typed")
			}
		}
	}
}

func visitUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.PrefixUnaryOperator()
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	switch op {
	case "+":
		t := ctx.TypeOf(operand)
		if t != nil && isLikeWithConstraint(t, "number") {
			ctx.Report(n, "redundant +x — value is already number-typed")
		}
	case "!":
		// `!!x` where x is boolean: detect when operand is itself a !x.
		if operand.Kind() == wrapperchecker.KindPrefixUnaryExpression && operand.PrefixUnaryOperator() == "!" {
			inner := operand.FirstChild()
			if inner == nil {
				return
			}
			it := ctx.TypeOf(inner)
			if it != nil && isLikeWithConstraint(it, "boolean") {
				ctx.Report(n, "redundant !! — value is already boolean-typed")
			}
		}
	case "~":
		// `~~x` where x is number-like AND known to be an integer.
		// `~~` truncates to int32 — it's only redundant when the value
		// is already an integer (a non-integer literal like 1.5 would
		// actually change).
		if operand.Kind() == wrapperchecker.KindPrefixUnaryExpression && operand.PrefixUnaryOperator() == "~" {
			inner := operand.FirstChild()
			if inner == nil {
				return
			}
			it := ctx.TypeOf(inner)
			if it != nil && isAllIntegerLiterals(it) {
				ctx.Report(n, "redundant ~~ — value is already number-typed")
			}
		}
	}
}

func isPrimitiveCtorName(s string) bool {
	switch s {
	case "String", "Number", "Boolean", "BigInt":
		return true
	}
	return false
}

func primName(s string) string {
	switch s {
	case "String":
		return "string"
	case "Number":
		return "number"
	case "Boolean":
		return "boolean"
	case "BigInt":
		return "bigint"
	}
	return s
}

func checkPrimitive(t *wrapperchecker.Type, ctorName string) bool {
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	switch ctorName {
	case "String":
		return isLikeWithConstraint(t, "string")
	case "Number":
		return isLikeWithConstraint(t, "number")
	case "Boolean":
		return isLikeWithConstraint(t, "boolean")
	case "BigInt":
		return isLikeWithConstraint(t, "bigint")
	}
	return false
}

// isLikeWithConstraint reports whether t is the requested primitive
// kind, walking type-parameter base constraints when needed (so
// `T extends string` counts as string-like).
func isLikeWithConstraint(t *wrapperchecker.Type, kind string) bool {
	if t == nil || t.IsAny() || t.IsUnknown() {
		return false
	}
	if matchesPrimitive(t, kind) {
		return true
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isLikeWithConstraint(c, kind)
		}
	}
	return false
}

func matchesPrimitive(t *wrapperchecker.Type, kind string) bool {
	switch kind {
	case "string":
		return t.IsStringLike()
	case "number":
		return t.IsNumberLike()
	case "boolean":
		return t.IsBooleanLike()
	case "bigint":
		return t.IsBigIntLike()
	}
	return false
}

func propertyAccessName(n *wrapperchecker.Node) string {
	if n == nil || n.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return ""
	}
	var name string
	depth := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if depth == 1 && c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		depth++
		return false
	})
	return name
}

// isAllIntegerLiterals reports whether t is a number-literal type with
// an integer value, or a union of such types.
func isAllIntegerLiterals(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isIntegerLiteral(m) {
				return false
			}
		}
		return true
	}
	return isIntegerLiteral(t)
}

func isIntegerLiteral(t *wrapperchecker.Type) bool {
	v, ok := t.NumericLiteralValue()
	if !ok {
		return false
	}
	return v == float64(int64(v))
}

func isEmptyStringLiteral(n *wrapperchecker.Node) bool {
	if n.Kind() == wrapperchecker.KindStringLiteral {
		return n.LiteralText() == ""
	}
	if n.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return n.LiteralText() == ""
	}
	return false
}
