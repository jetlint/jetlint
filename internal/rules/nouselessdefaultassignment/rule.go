// Package nouselessdefaultassignment implements the no-useless-default-assignment
// rule: flag default-assignments where the default literally cannot be
// reached because the destructured/assigned target's type excludes
// undefined.
package nouselessdefaultassignment

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-useless-default-assignment"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBindingElement: visitBindingElement,
		wrapperchecker.KindParameter:      visitParameter,
	}
}

// callbackContextualParamType returns the contextual signature's
// parameter type at paramIndex when fn is a function expression or
// arrow used as a callback argument. Mirrors typescript-eslint's
// upstream lookup: signatures[0].getParameters()[paramIndex]. Returns
// nil for top-level functions, methods, etc. — only callback-style
// usages have a contextual signature.
func callbackContextualParamType(ctx *engine.Context, fn *wrapperchecker.Node, paramIndex int) *wrapperchecker.Type {
	if fn == nil {
		return nil
	}
	switch fn.Kind() {
	case wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression:
	default:
		return nil
	}
	ct := ctx.Checker().ContextualTypeOf(fn)
	if ct == nil {
		return nil
	}
	sigs := ct.CallSignatures()
	if len(sigs) == 0 {
		return nil
	}
	sig := sigs[0]
	// Rest-parameter signatures (`(...args: T[]) => void`) bind any
	// number of arguments to the rest slot — the contextual type at
	// paramIndex is the element type, not the parameter type. Mirror
	// upstream's check that bails on rest params.
	if sig.HasRestParameter() {
		return nil
	}
	types := sig.ParameterTypes()
	if paramIndex >= len(types) {
		return nil
	}
	return types[paramIndex]
}

// fnParamIndex returns the 0-based position of param within fn's
// parameter list, or -1 when not found.
func fnParamIndex(fn *wrapperchecker.Node, param *wrapperchecker.Node) int {
	if fn == nil || param == nil {
		return -1
	}
	idx := 0
	found := -1
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindParameter {
			return false
		}
		if c.Pos() == param.Pos() && c.SourceText() == param.SourceText() {
			found = idx
			return true
		}
		idx++
		return false
	})
	return found
}

// visitBindingElement flags two destructuring-default shapes upstream
// reports as useless:
//
//   - default is the bare identifier `undefined` — equivalent to no
//     default at all (`{ a = undefined }`, `[a = undefined]`)
//   - object-pattern slot whose source-type property excludes
//     undefined (`{ a = '' }: { a: string }`)
//
// Array-pattern source checks are skipped: the upstream rule defers to
// noUncheckedIndexedAccess to avoid flagging `[a = ''] = stringArr`
// where the element type lies about runtime out-of-bounds reads.
func visitBindingElement(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.BindingElementInitializer()
	if init == nil {
		return
	}
	if isUndefinedIdentifier(init) {
		ctx.Report(init, "default of `undefined` is equivalent to no default — remove it")
		return
	}
	pattern := n.Parent()
	if pattern == nil {
		return
	}
	switch pattern.Kind() {
	case wrapperchecker.KindObjectBindingPattern:
		// Always safe to check by property type.
	case wrapperchecker.KindArrayBindingPattern:
		// Only check when source is a tuple — non-tuple array element
		// types lie about runtime out-of-bounds reads under the
		// default tsconfig (no noUncheckedIndexedAccess).
		srcOuter := destructuringPatternSourceType(ctx, pattern)
		if srcOuter == nil || !srcOuter.IsTupleType() {
			return
		}
	default:
		return
	}
	srcT := destructuringSlotType(ctx, n)
	if srcT == nil {
		return
	}
	if typeIncludesUndefined(srcT) {
		return
	}
	ctx.Report(init, "default-assignment is unreachable: this slot excludes undefined")
}

// isUndefinedIdentifier reports whether n is the bare `undefined`
// identifier — the literal default that's equivalent to having no
// default at all.
func isUndefinedIdentifier(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	return n.Kind() == wrapperchecker.KindIdentifier && n.LiteralText() == "undefined"
}

// visitParameter flags two parameter-default shapes:
//
//   - default is the bare identifier `undefined` — the typescript
//     `?:` optional syntax is preferred over `: T = undefined`
//   - the function is a callback whose contextual signature has the
//     parameter at the same position required (non-optional, not
//     rest, type excludes undefined) — the default never fires
//
// Top-level functions, methods, and arrow-bound vars without a
// contextual signature are skipped — there the default genuinely
// fires when the function is called with no argument.
func visitParameter(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.ParameterInitializer()
	if init == nil {
		return
	}
	if isUndefinedIdentifier(init) {
		ctx.Report(init, "default of `undefined` is equivalent to no default — use `?:` instead")
		return
	}
	fn := n.Parent()
	idx := fnParamIndex(fn, n)
	if idx < 0 {
		return
	}
	t := callbackContextualParamType(ctx, fn, idx)
	if t == nil {
		return
	}
	if t.IsAny() || t.IsUnknown() {
		return
	}
	if typeIncludesUndefined(t) {
		return
	}
	ctx.Report(init, "default-assignment is unreachable: contextual parameter type excludes undefined")
}

// destructuringSlotType returns the source-side type at the slot a
// BindingElement is bound to. For object destructuring this is the
// property type; for array destructuring this is the element type at
// the slot's index. Returns nil when the source type is unknown or
// the slot can't be resolved.
func destructuringSlotType(ctx *engine.Context, be *wrapperchecker.Node) *wrapperchecker.Type {
	pattern := be.Parent()
	if pattern == nil {
		return nil
	}
	srcT := destructuringPatternSourceType(ctx, pattern)
	if srcT == nil {
		return nil
	}
	switch pattern.Kind() {
	case wrapperchecker.KindObjectBindingPattern:
		key := bindingElementKey(be)
		if key == "" {
			return nil
		}
		// Property missing from any union arm makes the destructure
		// undefined-bearing — even if `srcT.PropertyType` returns a
		// non-nullable type from the apparent-property merge.
		if srcT.IsUnion() {
			for _, m := range srcT.UnionMembers() {
				if m.PropertyType(key) == nil {
					return nil
				}
			}
		}
		// Optional property (`{ a?: T }`) — the slot can be undefined
		// at runtime regardless of T's declared type. Treat as
		// undefined-bearing UNLESS the source expression is a
		// conditional / logical chain whose every branch supplies the
		// key directly. TS marks the property as optional when ANY
		// branch lacks it, which can produce false positives on the
		// "all branches have it" cases that upstream still flags.
		if sym := srcT.PropertySymbol(key); sym != nil && sym.IsOptional() {
			if !propertyInAllBranches(destructuringInit(pattern), key) {
				return nil
			}
		}
		return srcT.PropertyType(key)
	case wrapperchecker.KindArrayBindingPattern:
		idx := arrayBindingIndex(pattern, be)
		if idx < 0 {
			return nil
		}
		if srcT.IsTupleType() {
			args := srcT.TypeArguments()
			if idx < len(args) {
				return args[idx]
			}
			return nil
		}
		return srcT.NumberIndexType()
	}
	return nil
}

// destructuringPatternSourceType resolves the type feeding the
// destructuring pattern. Walks to the parent VariableDeclaration,
// Parameter, or enclosing BindingElement to find the source.
func destructuringPatternSourceType(ctx *engine.Context, pattern *wrapperchecker.Node) *wrapperchecker.Type {
	p := pattern.Parent()
	if p == nil {
		return nil
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		if init := p.VariableDeclarationInitializer(); init != nil {
			return ctx.TypeOf(init)
		}
	case wrapperchecker.KindParameter:
		if annot := p.ParameterTypeAnnotation(); annot != nil {
			return ctx.Checker().TypeFromTypeNode(annot)
		}
	case wrapperchecker.KindBindingElement:
		return destructuringSlotType(ctx, p)
	}
	return nil
}

// bindingElementKey returns the property name a BindingElement reads
// from for object destructuring. Falls back to the binding-target
// identifier when no explicit `: alias` is used.
func bindingElementKey(be *wrapperchecker.Node) string {
	if pn := be.BindingElementPropertyName(); pn != nil {
		switch pn.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return pn.LiteralText()
		}
	}
	if name := be.BindingElementName(); name != nil && name.Kind() == wrapperchecker.KindIdentifier {
		return name.LiteralText()
	}
	return ""
}

// arrayBindingIndex returns the 0-based position of the BindingElement
// within an array binding pattern. Returns -1 for rest elements or
// when the element isn't found.
func arrayBindingIndex(pattern, target *wrapperchecker.Node) int {
	idx := 0
	found := -1
	pattern.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindBindingElement:
			if c.BindingElementIsRest() {
				idx++
				return false
			}
			if c.SourceText() == target.SourceText() && c.Pos() == target.Pos() {
				found = idx
				return true
			}
			idx++
		case wrapperchecker.KindOmittedExpression:
			idx++
		}
		return false
	})
	return found
}

// destructuringInit returns the initializer expression feeding the
// destructure pattern, walking up nested patterns to find the
// outermost source. Nil when the pattern isn't backed by a runtime
// expression (e.g. function parameters).
func destructuringInit(pattern *wrapperchecker.Node) *wrapperchecker.Node {
	cur := pattern
	for cur != nil {
		p := cur.Parent()
		if p == nil {
			return nil
		}
		switch p.Kind() {
		case wrapperchecker.KindVariableDeclaration:
			return p.VariableDeclarationInitializer()
		case wrapperchecker.KindBindingElement:
			cur = p.Parent()
			continue
		}
		return nil
	}
	return nil
}

// propertyInAllBranches reports whether expr is an object literal
// or a conditional/logical/parenthesized chain whose every leaf is
// an object literal carrying the named property. Mirrors upstream's
// `hasPropertyInAllBranches`.
func propertyInAllBranches(expr *wrapperchecker.Node, key string) bool {
	for expr != nil && expr.Kind() == wrapperchecker.KindParenthesizedExpression {
		expr = expr.FirstChild()
	}
	if expr == nil {
		return false
	}
	switch expr.Kind() {
	case wrapperchecker.KindObjectLiteralExpression:
		found := false
		expr.ForEachChild(func(c *wrapperchecker.Node) bool {
			if propertyAssignmentMatchesKey(c, key) {
				found = true
				return true
			}
			return false
		})
		return found
	case wrapperchecker.KindConditionalExpression:
		whenTrue, whenFalse := expr.ConditionalBranches()
		return propertyInAllBranches(whenTrue, key) && propertyInAllBranches(whenFalse, key)
	case wrapperchecker.KindBinaryExpression:
		// Logical-or/and/?? — both sides must supply the property.
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindBarBarToken,
			wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindQuestionQuestionToken:
			return propertyInAllBranches(expr.BinaryLeft(), key) &&
				propertyInAllBranches(expr.BinaryRight(), key)
		}
	}
	return false
}

// propertyAssignmentMatchesKey reports whether prop is a property
// assignment (`a: ...`), shorthand (`a`), method (`a() {}`), or
// computed property whose key text matches the given key.
func propertyAssignmentMatchesKey(prop *wrapperchecker.Node, key string) bool {
	if prop == nil {
		return false
	}
	switch prop.Kind() {
	case wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		// First child is the name node.
		var name *wrapperchecker.Node
		prop.ForEachChild(func(c *wrapperchecker.Node) bool {
			name = c
			return true
		})
		return propertyKeyText(name) == key
	}
	return false
}

// propertyKeyText returns the textual key of an object-literal
// property name node — handles bare identifiers, string literals,
// and `['key']` computed names whose body is a literal.
func propertyKeyText(name *wrapperchecker.Node) string {
	if name == nil {
		return ""
	}
	switch name.Kind() {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return name.LiteralText()
	}
	// Fallback for computed names like `['a']`: dig through children
	// and pull the first literal-like node's text.
	var inner *wrapperchecker.Node
	name.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral,
			wrapperchecker.KindIdentifier:
			inner = c
			return true
		}
		return false
	})
	if inner != nil {
		return inner.LiteralText()
	}
	return ""
}

// typeIncludesUndefined reports whether t (or any union arm) is
// undefined-bearing. Conservatively returns true for `any`/`unknown`
// since either inhabits undefined.
func typeIncludesUndefined(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeIncludesUndefined(m) {
				return true
			}
		}
		return false
	}
	return t.IsNullOrUndefined() && t.String() != "null"
}
