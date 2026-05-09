// Package nounsafeassignment implements the no-unsafe-assignment rule:
// flag where an `any` value is laundered into a more-specific typed
// slot — variable declarations, assignment expressions, object
// literal property values, and array elements.
package nounsafeassignment

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-assignment"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visitVariableDeclaration,
		wrapperchecker.KindBinaryExpression:    visitBinary,
		wrapperchecker.KindPropertyAssignment:          visitPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment: visitShorthandProperty,
		wrapperchecker.KindJsxAttribute:                visitJsxAttribute,
		wrapperchecker.KindParameter:           visitParameter,
		wrapperchecker.KindPropertyDeclaration: visitPropertyDeclaration,
		wrapperchecker.KindSpreadElement:       visitSpreadElement,
	}
}

func visitParameter(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.ParameterInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	annot := n.ParameterTypeAnnotation()
	var lhs *wrapperchecker.Type
	if annot != nil {
		lhs = ctx.Checker().TypeFromTypeNode(annot)
	}
	bindName := n.ParameterName()
	if didReport := check(ctx, init, rhs, lhs); didReport {
		return
	}
	walkDestructure(ctx, bindName, rhs)
}

func visitPropertyDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.PropertyDeclarationInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	annot := n.PropertyDeclarationType()
	var lhs *wrapperchecker.Type
	if annot != nil {
		lhs = ctx.Checker().TypeFromTypeNode(annot)
	}
	check(ctx, init, rhs, lhs)
}

func visitVariableDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	annot := n.VariableDeclarationType()
	var lhs *wrapperchecker.Type
	if annot != nil {
		lhs = ctx.Checker().TypeFromTypeNode(annot)
	}
	bindName := n.VariableDeclarationName()
	// `const x = ...` — basic any-launder check on the initializer.
	if bindName != nil && bindName.Kind() == wrapperchecker.KindIdentifier {
		check(ctx, init, rhs, lhs)
		return
	}
	// Destructuring shapes (`const [x] = ...`, `const {x} = ...`) walk
	// the binding tree against the initializer's type.
	if didReport := check(ctx, init, rhs, lhs); didReport {
		return
	}
	walkDestructure(ctx, bindName, rhs)
}

// walkDestructure mirrors upstream's checkArrayDestructure /
// checkObjectDestructure: the receiver pattern is matched against the
// sender's type and reports occur on the receiver element nodes when
// the corresponding sender slot is `any`. Reports stop descending past
// a flagged level so the same value isn't reported for every nested
// rewrap (e.g., `[[[x]]] = [any]` reports once on the outer pattern).
func walkDestructure(ctx *engine.Context, recv *wrapperchecker.Node, sender *wrapperchecker.Type) {
	if recv == nil || sender == nil {
		return
	}
	switch recv.Kind() {
	case wrapperchecker.KindArrayBindingPattern:
		walkArrayPattern(ctx, recv, sender)
	case wrapperchecker.KindObjectBindingPattern:
		walkObjectPattern(ctx, recv, sender)
	}
}

func walkArrayPattern(ctx *engine.Context, pattern *wrapperchecker.Node, sender *wrapperchecker.Type) {
	if isAnyArrayType(sender) {
		ctx.Report(pattern, "unsafe array destructuring of an `any` array value")
		return
	}
	if !sender.IsTupleType() {
		return
	}
	tupleArgs := sender.TypeArguments()
	pattern.ForEachChild(func(c *wrapperchecker.Node) bool {
		idx := indexOfBindingElement(pattern, c)
		if idx < 0 || idx >= len(tupleArgs) {
			return false
		}
		if c.Kind() != wrapperchecker.KindBindingElement {
			return false
		}
		if c.BindingElementIsRest() {
			return false
		}
		elemType := tupleArgs[idx]
		name := c.BindingElementName()
		if name == nil {
			return false
		}
		if elemType != nil && elemType.IsAny() {
			ctx.Report(name, "unsafe array destructuring of a tuple element with an `any` value")
			return false
		}
		switch name.Kind() {
		case wrapperchecker.KindArrayBindingPattern:
			walkArrayPattern(ctx, name, elemType)
		case wrapperchecker.KindObjectBindingPattern:
			walkObjectPattern(ctx, name, elemType)
		}
		return false
	})
}

func walkObjectPattern(ctx *engine.Context, pattern *wrapperchecker.Node, sender *wrapperchecker.Type) {
	pattern.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindBindingElement {
			return false
		}
		if c.BindingElementIsRest() {
			return false
		}
		key := bindingElementKey(c)
		if key == "" {
			return false
		}
		propType := sender.PropertyType(key)
		if propType == nil {
			return false
		}
		name := c.BindingElementName()
		if name == nil {
			return false
		}
		if propType.IsAny() {
			ctx.Report(name, "unsafe object destructuring of a property with an `any` value")
			return false
		}
		switch name.Kind() {
		case wrapperchecker.KindArrayBindingPattern:
			walkArrayPattern(ctx, name, propType)
		case wrapperchecker.KindObjectBindingPattern:
			walkObjectPattern(ctx, name, propType)
		}
		return false
	})
}

// indexOfBindingElement returns the position of a child within its
// ArrayBindingPattern parent, counting binding elements and omitted
// holes. ArrayBindingPattern children alternate between BindingElement
// (a target) and OmittedExpression (a hole from `[, x]`).
func indexOfBindingElement(pattern, target *wrapperchecker.Node) int {
	idx := -1
	found := -1
	pattern.ForEachChild(func(c *wrapperchecker.Node) bool {
		idx++
		if sameNode(c, target) {
			found = idx
			return true
		}
		return false
	})
	return found
}

func sameNode(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	af, asl, asc, ael, aec := a.SourceRange()
	bf, bsl, bsc, bel, bec := b.SourceRange()
	return af == bf && asl == bsl && asc == bsc && ael == bel && aec == bec
}

// bindingElementKey returns the property name that an
// ObjectBindingPattern element binds. Falls back to the binding's own
// identifier name for shorthand `{ a }` (no propertyName). Handles
// computed property names whose expression is a string-literal-like
// node (`['x']`, `` [`x`] ``).
func bindingElementKey(elem *wrapperchecker.Node) string {
	if pn := elem.BindingElementPropertyName(); pn != nil {
		switch pn.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPrivateIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral,
			wrapperchecker.KindNumericLiteral:
			return pn.LiteralText()
		}
		// Computed names are ComputedPropertyName nodes; peek inside
		// for a literal expression we can read.
		inner := pn.FirstChild()
		if inner != nil {
			switch inner.Kind() {
			case wrapperchecker.KindStringLiteral,
				wrapperchecker.KindNoSubstitutionTemplateLiteral,
				wrapperchecker.KindNumericLiteral:
				return inner.LiteralText()
			}
		}
		return ""
	}
	name := elem.BindingElementName()
	if name == nil {
		return ""
	}
	if name.Kind() == wrapperchecker.KindIdentifier {
		return name.LiteralText()
	}
	return ""
}

// isAnyArrayType reports whether t is the array form of `any` —
// `any[]` or `Array<any>`. Used to flag the entire destructuring
// receiver in one shot.
func isAnyArrayType(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if elem := t.ArrayElementType(); elem != nil {
		return elem.IsAny()
	}
	return false
}

func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.BinaryOperatorKind() != wrapperchecker.KindEqualsToken {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	rhs := ctx.TypeOf(right)
	lhs := ctx.TypeOf(left)
	if didReport := check(ctx, right, rhs, lhs); didReport {
		return
	}
	// Destructuring assignment targets: in TypeScript's AST these are
	// ArrayLiteralExpression / ObjectLiteralExpression on the LHS,
	// not formal binding patterns. Walk them like patterns against the
	// rhs's type so per-position any leaks are reported.
	walkAssignmentTarget(ctx, unwrapParens(left), rhs)
}

func unwrapParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

func walkAssignmentTarget(ctx *engine.Context, target *wrapperchecker.Node, sender *wrapperchecker.Type) {
	if target == nil || sender == nil {
		return
	}
	switch target.Kind() {
	case wrapperchecker.KindArrayLiteralExpression:
		walkArrayAssignmentTarget(ctx, target, sender)
	case wrapperchecker.KindObjectLiteralExpression:
		walkObjectAssignmentTarget(ctx, target, sender)
	}
}

func walkArrayAssignmentTarget(ctx *engine.Context, target *wrapperchecker.Node, sender *wrapperchecker.Type) {
	if isAnyArrayType(sender) {
		ctx.Report(target, "unsafe array destructuring of an `any` array value")
		return
	}
	if !sender.IsTupleType() {
		return
	}
	tupleArgs := sender.TypeArguments()
	idx := -1
	target.ForEachChild(func(c *wrapperchecker.Node) bool {
		idx++
		if idx >= len(tupleArgs) {
			return false
		}
		if c.Kind() == wrapperchecker.KindOmittedExpression ||
			c.Kind() == wrapperchecker.KindSpreadElement {
			return false
		}
		elemType := tupleArgs[idx]
		if elemType != nil && elemType.IsAny() {
			ctx.Report(c, "unsafe array destructuring of a tuple element with an `any` value")
			return false
		}
		// Recurse into nested array/object assignment targets.
		walkAssignmentTarget(ctx, unwrapParens(c), elemType)
		return false
	})
}

func walkObjectAssignmentTarget(ctx *engine.Context, target *wrapperchecker.Node, sender *wrapperchecker.Type) {
	target.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPropertyAssignment:
			key, value := propertyAssignmentKeyValue(c)
			if key == "" || value == nil {
				return false
			}
			propType := sender.PropertyType(key)
			if propType == nil {
				return false
			}
			if propType.IsAny() {
				ctx.Report(value, "unsafe object destructuring of a property with an `any` value")
				return false
			}
			walkAssignmentTarget(ctx, unwrapParens(value), propType)
		case wrapperchecker.KindShorthandPropertyAssignment:
			name := c.FirstChild()
			if name == nil {
				return false
			}
			key := name.LiteralText()
			if key == "" {
				return false
			}
			propType := sender.PropertyType(key)
			if propType == nil {
				return false
			}
			if propType.IsAny() {
				ctx.Report(c, "unsafe object destructuring of a property with an `any` value")
			}
		}
		return false
	})
}

func propertyAssignmentKeyValue(n *wrapperchecker.Node) (string, *wrapperchecker.Node) {
	var key string
	var value *wrapperchecker.Node
	seenKey := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenKey {
			switch c.Kind() {
			case wrapperchecker.KindIdentifier,
				wrapperchecker.KindPrivateIdentifier,
				wrapperchecker.KindStringLiteral,
				wrapperchecker.KindNoSubstitutionTemplateLiteral,
				wrapperchecker.KindNumericLiteral:
				key = c.LiteralText()
				seenKey = true
				return false
			}
			// Computed property name (`[expr]:`) — its node has a
			// single inner expression. Identify by no leaf-name match
			// and presence of a single child that's a literal.
			if !seenKey && c.Kind() != wrapperchecker.KindIdentifier {
				inner := c.FirstChild()
				if inner != nil {
					switch inner.Kind() {
					case wrapperchecker.KindStringLiteral,
						wrapperchecker.KindNoSubstitutionTemplateLiteral,
						wrapperchecker.KindNumericLiteral:
						key = inner.LiteralText()
						seenKey = true
						return false
					}
				}
			}
		} else {
			value = c
			return true
		}
		return false
	})
	return key, value
}

func visitPropertyAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip property assignments inside an object-pattern destructuring
	// target (e.g., the `x: y` in `({ x: y } = ...)`); those are
	// handled by the destructure walk on the assignment node, where
	// the sender's type drives the report position.
	if isInObjectAssignmentPattern(n) {
		return
	}
	init := n.PropertyInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	if rhs == nil {
		return
	}
	// Even when no contextual type is available, an `any`-typed
	// initializer is still flagged — there's no receiver type that
	// would suppress the launder. Pass nil lhs through to the basic
	// check, which handles the any → unknown escape and any → any
	// pass-through.
	lhs := ctx.Checker().ContextualTypeOf(init)
	check(ctx, init, rhs, lhs)
}

func visitShorthandProperty(ctx *engine.Context, n *wrapperchecker.Node) {
	if isInObjectAssignmentPattern(n) {
		return
	}
	// `{ y = 1 }` (shorthand with a default initializer) only makes
	// sense as a destructuring binding — in a value-position object
	// literal it's a syntax error. Upstream skips this shape from the
	// property check; we follow suit so the surrounding fixture's
	// "this is not checked" comment holds.
	if shorthandHasDefault(n) {
		return
	}
	id := n.FirstChild()
	if id == nil {
		return
	}
	rhs := ctx.TypeOf(id)
	if rhs == nil {
		return
	}
	lhs := ctx.Checker().ContextualTypeOf(id)
	check(ctx, n, rhs, lhs)
}

func shorthandHasDefault(n *wrapperchecker.Node) bool {
	count := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		count++
		return false
	})
	return count > 1
}

// visitJsxAttribute handles `<Foo a={1 as any} />`: when an attribute
// value's expression is `any`-typed but the attribute's contextual
// type is more specific, flag the assignment.
func visitJsxAttribute(ctx *engine.Context, n *wrapperchecker.Node) {
	var jsxExpr *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindJsxExpression {
			jsxExpr = c
			return true
		}
		return false
	})
	if jsxExpr == nil {
		return
	}
	value := jsxExpr.FirstChild()
	if value == nil {
		return
	}
	rhs := ctx.TypeOf(value)
	lhs := ctx.Checker().ContextualTypeOf(value)
	check(ctx, value, rhs, lhs)
}

// isInObjectAssignmentPattern reports whether a PropertyAssignment is
// nested inside an ObjectLiteralExpression that's being used as the
// LHS of a destructuring assignment (rather than as a value
// expression). Detection: walk up through ObjectLiteralExpression /
// ArrayLiteralExpression / PropertyAssignment until we hit a
// BinaryExpression whose left side is the literal — if the literal is
// the left of `=`, we're a destructuring target.
func isInObjectAssignmentPattern(n *wrapperchecker.Node) bool {
	cur := n.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindObjectLiteralExpression,
			wrapperchecker.KindArrayLiteralExpression,
			wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindParenthesizedExpression:
			next := cur.Parent()
			if next == nil {
				return false
			}
			if next.Kind() == wrapperchecker.KindBinaryExpression &&
				next.BinaryOperatorKind() == wrapperchecker.KindEqualsToken &&
				sameNode(next.BinaryLeft(), cur) {
				return true
			}
			cur = next
		default:
			return false
		}
	}
	return false
}

func visitSpreadElement(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil || parent.Kind() != wrapperchecker.KindArrayLiteralExpression {
		return
	}
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if t.IsAny() || isAnyArrayType(t) {
		ctx.Report(n, "unsafe spread of an `any` value in an array")
	}
}

// check returns true when it reported, mirroring upstream's
// "didReport" return that suppresses destructure-walks once the basic
// any-launder has already fired.
func check(ctx *engine.Context, src *wrapperchecker.Node, rhs, lhs *wrapperchecker.Type) bool {
	if rhs == nil {
		return false
	}
	if lhs != nil && lhs.IsUnknown() {
		return false
	}
	if rhs.IsAny() {
		if lhs != nil && lhs.IsAny() {
			return false
		}
		ctx.Report(src, "unsafe assignment of an `any` value")
		return true
	}
	if lhs == nil {
		return false
	}
	if lhs.IsAny() {
		return false
	}
	if !assignmentIsUnsafe(rhs, lhs, 8) {
		return false
	}
	if !rhs.IsAny() && !exprHasExplicitAnyKeyword(src) {
		return false
	}
	ctx.Report(src, "unsafe assignment of an `any` value to a more specific declared type")
	return true
}

func exprHasExplicitAnyKeyword(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindAnyKeyword {
		return true
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if exprHasExplicitAnyKeyword(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

func assignmentIsUnsafe(rhs, lhs *wrapperchecker.Type, depth int) bool {
	if rhs == nil || depth <= 0 {
		return false
	}
	if rhs.IsAny() {
		if lhs.IsAny() || lhs.IsUnknown() {
			return false
		}
		return true
	}
	if rhs.IsUnion() {
		for _, m := range rhs.UnionMembers() {
			if assignmentIsUnsafe(m, lhs, depth-1) {
				return true
			}
		}
		return false
	}
	rhsArgs := rhs.TypeArguments()
	lhsArgs := lhs.TypeArguments()
	if len(rhsArgs) > 0 && len(rhsArgs) == len(lhsArgs) &&
		rhs.SymbolName() == lhs.SymbolName() && rhs.SymbolName() != "" {
		for i := range rhsArgs {
			if assignmentIsUnsafe(rhsArgs[i], lhsArgs[i], depth-1) {
				return true
			}
		}
	}
	return false
}
