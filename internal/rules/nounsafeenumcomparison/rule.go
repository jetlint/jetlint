// Package nounsafeenumcomparison implements the no-unsafe-enum-comparison
// rule: flag a comparison where one side is an enum and the other isn't
// the same enum (e.g. `Fruit.Apple === 0`).
package nounsafeenumcomparison

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unsafe-enum-comparison"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
		wrapperchecker.KindSwitchStatement:  visitSwitch,
	}
}

func visitSwitch(ctx *engine.Context, n *wrapperchecker.Node) {
	disc := n.SwitchExpression()
	if disc == nil {
		return
	}
	discT := ctx.TypeOf(disc)
	if discT == nil {
		return
	}
	if isAlwaysAcceptable(discT) {
		return
	}
	// Walk case clauses — flag when either side carries an enum and the
	// other side doesn't share the enum's identity.
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		walkCaseClauses(ctx, c, discT)
		return false
	})
}

func walkCaseClauses(ctx *engine.Context, c *wrapperchecker.Node, discT *wrapperchecker.Type) {
	c.ForEachChild(func(cc *wrapperchecker.Node) bool {
		if cc.Kind() != wrapperchecker.KindCaseClause {
			return false
		}
		expr := cc.CaseExpression()
		if expr == nil {
			return false
		}
		caseT := ctx.TypeOf(expr)
		if caseT == nil {
			return false
		}
		if isAlwaysAcceptable(caseT) || isAlwaysAcceptable(discT) {
			return false
		}
		discHasEnum := containsEnum(discT)
		caseHasEnum := containsEnum(caseT)
		if !discHasEnum && !caseHasEnum {
			return false
		}
		if discHasEnum && caseHasEnum {
			if sameEnumIdentity(discT, caseT) {
				return false
			}
			ctx.Report(cc, "case label is from a different enum than the switch discriminant")
			return false
		}
		enumSide, otherSide := discT, caseT
		if caseHasEnum {
			enumSide, otherSide = caseT, discT
		}
		if otherCompatibleWithEnumUnion(otherSide, enumSide) {
			return false
		}
		otherKind := classifyPrimitive(otherSide)
		if otherKind == "" {
			return false
		}
		if enumKind := classifyEnumValueKind(enumSide); enumKind != "" && otherKind != enumKind {
			return false
		}
		ctx.Report(cc, "case label compares against a value of a different type than the switch discriminant's enum")
		return false
	})
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
	otherKind := classifyPrimitive(otherSide)
	if otherKind == "" {
		// Object / function / other non-primitive types can never equal
		// an enum value at runtime — the comparison is dead, not unsafe.
		return
	}
	if enumKind := classifyEnumValueKind(enumSide); enumKind != "" && otherKind != enumKind {
		// Different primitive kinds — same reasoning.
		return
	}
	if otherKind == "boolean" || otherKind == "bigint" {
		// Enums can only contain number or string members; the comparison
		// is dead, not unsafe.
		return
	}
	ctx.Report(n, "comparing an enum to a non-enum value; either use the enum's member or compare two enums of the same type")
}

// classifyEnumValueKind reports whether the enum's underlying value
// type is "string" or "number" (or "" if neither/mixed).
func classifyEnumValueKind(t *wrapperchecker.Type) string {
	var kinds []string
	for _, m := range t.UnionMembers() {
		if !m.IsEnumLike() {
			continue
		}
		switch {
		case m.IsStringLike():
			kinds = append(kinds, "string")
		case m.IsNumberLike():
			kinds = append(kinds, "number")
		}
	}
	if len(kinds) == 0 {
		return ""
	}
	first := kinds[0]
	for _, k := range kinds[1:] {
		if k != first {
			return ""
		}
	}
	return first
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

// otherCompatibleWithEnumUnion reports whether `other` is assignable
// to one of the non-enum union members of `enumSide` (e.g.
// enumSide=`Fruit | -1` and other=`-1`). Branded intersections like
// `string & { __brand: void }` reject plain string literals, so the
// comparison is still unsafe.
func otherCompatibleWithEnumUnion(other, enumSide *wrapperchecker.Type) bool {
	if !enumSide.IsUnion() {
		return false
	}
	for _, m := range enumSide.UnionMembers() {
		if m.IsEnumLike() {
			continue
		}
		if other.IsAssignableTo(m) {
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
	if t.IsIntersection() {
		// `number & {}` (branded primitive) — pick any primitive member.
		for _, m := range t.IntersectionMembers() {
			if c := classifyPrimitive(m); c != "" {
				return c
			}
		}
	}
	return ""
}
