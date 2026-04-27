// Package nounsafeenumcomparison implements the no-unsafe-enum-comparison
// rule: flag a comparison where one side is an enum and the other isn't
// the same enum (e.g. `Fruit.Apple === 0`).
package nounsafeenumcomparison

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-enum-comparison"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isComparison(n.BinaryOperatorKind()) {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	leftT := ctx.TypeOf(left)
	rightT := ctx.TypeOf(right)
	if leftT == nil || rightT == nil {
		return
	}
	leftHasEnum := containsEnum(leftT)
	rightHasEnum := containsEnum(rightT)
	if !leftHasEnum && !rightHasEnum {
		return
	}
	if isAlwaysAcceptable(leftT) || isAlwaysAcceptable(rightT) {
		return
	}
	if leftHasEnum && rightHasEnum {
		if sameEnumIdentity(leftT, rightT) {
			return
		}
		ctx.Report(n, "comparing different enums has no shared values")
		return
	}
	enumSide, otherSide := leftT, rightT
	if rightHasEnum {
		enumSide, otherSide = rightT, leftT
	}
	if otherCompatibleWithEnumUnion(otherSide, enumSide) {
		return
	}
	ctx.Report(n, "comparing an enum to a non-enum value; either use the enum's member or compare two enums of the same type")
}

func isComparison(op wrapperchecker.Kind) bool {
	switch op {
	case wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindLessThanToken,
		wrapperchecker.KindLessThanEqualsToken,
		wrapperchecker.KindGreaterThanToken,
		wrapperchecker.KindGreaterThanEqualsToken:
		return true
	}
	return false
}

func containsEnum(t *wrapperchecker.Type) bool {
	if t.IsEnumLike() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsEnumLike() {
				return true
			}
		}
	}
	return false
}

func isAlwaysAcceptable(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() || t.IsNullOrUndefined() {
		return true
	}
	return false
}

func sameEnumIdentity(a, b *wrapperchecker.Type) bool {
	for _, ea := range collectEnumNames(a) {
		for _, eb := range collectEnumNames(b) {
			if ea != "" && ea == eb {
				return true
			}
		}
	}
	return false
}

func collectEnumNames(t *wrapperchecker.Type) []string {
	var out []string
	if t.IsEnumLike() {
		if name := enumName(t); name != "" {
			out = append(out, name)
		}
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsEnumLike() {
				if name := enumName(m); name != "" {
					out = append(out, name)
				}
			}
		}
	}
	return out
}

func enumName(t *wrapperchecker.Type) string {
	if name := t.EnumName(); name != "" {
		return name
	}
	if name := t.SymbolName(); name != "" {
		return name
	}
	return t.AliasSymbolName()
}

// otherCompatibleWithEnumUnion reports whether `other` shares a
// primitive kind with one of the non-enum union members of `enumSide`
// (e.g. enumSide=`Fruit | -1` and other=`-1`).
func otherCompatibleWithEnumUnion(other, enumSide *wrapperchecker.Type) bool {
	otherKind := classifyPrimitive(other)
	if otherKind == "" {
		return false
	}
	for _, m := range enumSide.UnionMembers() {
		if m.IsEnumLike() {
			continue
		}
		if classifyPrimitive(m) == otherKind {
			return true
		}
	}
	return false
}

func classifyPrimitive(t *wrapperchecker.Type) string {
	if t.IsStringLike() {
		return "string"
	}
	if t.IsNumberLike() {
		return "number"
	}
	if t.IsBooleanLike() {
		return "boolean"
	}
	if t.IsBigIntLike() {
		return "bigint"
	}
	if t.IsUnion() {
		var seen string
		for _, m := range t.UnionMembers() {
			c := classifyPrimitive(m)
			if c == "" {
				continue
			}
			if seen == "" {
				seen = c
				continue
			}
			if seen != c {
				return ""
			}
		}
		return seen
	}
	return ""
}
