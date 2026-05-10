// Package preferreadonly implements the prefer-readonly rule: flag
// private class fields that are never reassigned after construction.
package preferreadonly

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-readonly"

// Options configures the rule. `OnlyInlineLambdas` (the upstream
// option) restricts the readonly suggestion to fields whose initializer
// is an arrow function, leaving non-lambda fields alone.
type Options struct {
	OnlyInlineLambdas bool
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return &rule{} }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyDeclaration: r.visit,
		wrapperchecker.KindParameter:           r.visitParam,
	}
}

// visitParam handles constructor parameter properties — `constructor
// (private foo = 1) {}` synthesizes a class field with the same
// modifiers and is subject to the same readonly check.
func (r *rule) visitParam(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.HasPrivateModifier() {
		return
	}
	if n.HasReadonlyModifier() {
		return
	}
	if r.opts.OnlyInlineLambdas && !propertyHasArrowInitializer(n) {
		return
	}
	parent := n.Parent()
	if parent == nil || parent.Kind() != wrapperchecker.KindConstructor {
		return
	}
	cls := parent.Parent()
	if cls == nil {
		return
	}
	if cls.Kind() != wrapperchecker.KindClassDeclaration && cls.Kind() != wrapperchecker.KindClassExpression {
		return
	}
	name := parameterName(n)
	if name == "" {
		return
	}
	if classWritesToProperty(cls, n, name) {
		return
	}
	ctx.Report(n, "private parameter property is never reassigned; declare it `readonly`")
}

// propertyHasArrowInitializer reports whether n's initializer (the
// expression after `=`) is an arrow function. Used to gate the
// `onlyInlineLambdas` carve-out.
func propertyHasArrowInitializer(n *wrapperchecker.Node) bool {
	found := false
	sawEquals := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if sawEquals {
			if c.Kind() == wrapperchecker.KindArrowFunction {
				found = true
			}
			return true
		}
		if c.Kind() == wrapperchecker.KindEqualsToken {
			sawEquals = true
		}
		return false
	})
	if found {
		return true
	}
	// Fallback: when EqualsToken is absent in the child stream, look
	// for an ArrowFunction directly under the declaration. PropertyDecl
	// children sometimes elide the `=` in tsgo's tree.
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindArrowFunction {
			found = true
			return true
		}
		return false
	})
	return found
}

func parameterName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.HasPrivateModifier() && !hasPrivateIdentifierName(n) {
		return
	}
	if n.HasReadonlyModifier() {
		return
	}
	if n.HasAccessorModifier() {
		// Auto-accessor fields desugar into a getter/setter pair —
		// declaring them readonly would silently drop the setter.
		return
	}
	if r.opts.OnlyInlineLambdas && !propertyHasArrowInitializer(n) {
		return
	}
	name := propertyName(n)
	if name == "" {
		return
	}
	cls := n.Parent()
	if cls == nil {
		return
	}
	if cls.Kind() != wrapperchecker.KindClassDeclaration && cls.Kind() != wrapperchecker.KindClassExpression {
		return
	}
	isStatic := n.HasStaticModifier()
	if isStatic {
		// Static fields are addressable as `ClassName.field`. Walk the
		// class once to see whether anything reassigns it that way; if
		// so, the field can't be readonly.
		clsName := classDeclarationName(cls)
		if clsName != "" && classWritesToStatic(cls, n, name, clsName) {
			return
		}
	} else if classWritesToProperty(cls, n, name) {
		return
	}
	ctx.Report(n, "private field is never reassigned; declare it `readonly`")
}

// classDeclarationName returns the identifier text of a class
// declaration or expression, or "" for unnamed class expressions.
func classDeclarationName(cls *wrapperchecker.Node) string {
	var name string
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// classWritesToStatic reports whether any code inside the class
// assigns to `ClassName.<name>` (or to `<alias>.<name>` for an alias
// of `typeof ClassName`) outside the field declaration itself. Walks
// every nested expression but stops at the field declaration so the
// initializer doesn't count as a reassignment.
func classWritesToStatic(cls, field *wrapperchecker.Node, name, clsName string) bool {
	written := false
	var walk func(n *wrapperchecker.Node, aliases map[string]bool)
	walk = func(n *wrapperchecker.Node, aliases map[string]bool) {
		if written || n == nil || n == field {
			return
		}
		// Methods, accessors, and the constructor open a new function
		// scope where `this` and class-name references rebind — collect
		// per-scope aliases so writes via type-asserted aliases (`const
		// that = {} as typeof Cls & { ... }`) flag correctly.
		switch n.Kind() {
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			scopedAliases := map[string]bool{}
			collectThisAliases(n, scopedAliases)
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c, scopedAliases)
				return written
			})
			return
		}
		if isStaticAssignmentTo(n, name, clsName) ||
			isStaticIncrementOf(n, name, clsName) ||
			isStaticDeleteOf(n, name, clsName) {
			written = true
			return
		}
		if isAliasAssignmentTo(n, name, aliases) ||
			isAliasIncrementOf(n, name, aliases) ||
			isAliasDeleteOf(n, name, aliases) {
			written = true
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c, aliases)
			return written
		})
	}
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c, nil)
		return written
	})
	return written
}

func isStaticAssignmentTo(n *wrapperchecker.Node, name, clsName string) bool {
	if n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken, wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken, wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken, wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken, wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
	default:
		return false
	}
	left := n.BinaryLeft()
	if left == nil || left.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if left.PropertyAccessName() != name {
		return false
	}
	recv := left.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && recv.LiteralText() == clsName
}

func isStaticIncrementOf(n *wrapperchecker.Node, name, clsName string) bool {
	switch n.Kind() {
	case wrapperchecker.KindPrefixUnaryExpression, wrapperchecker.KindPostfixUnaryExpression:
	default:
		return false
	}
	if op := n.PrefixUnaryOperator(); op != "++" && op != "--" {
		return false
	}
	target := n.FirstChild()
	if target == nil || target.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if target.PropertyAccessName() != name {
		return false
	}
	recv := target.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && recv.LiteralText() == clsName
}

func isStaticDeleteOf(n *wrapperchecker.Node, name, clsName string) bool {
	if n.Kind() != wrapperchecker.KindDeleteExpression {
		return false
	}
	target := n.FirstChild()
	if target == nil || target.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if target.PropertyAccessName() != name {
		return false
	}
	recv := target.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && recv.LiteralText() == clsName
}

func propertyName(n *wrapperchecker.Node) string {
	var name string
	var computedExpr *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if name != "" || computedExpr != nil {
			return true
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier, wrapperchecker.KindPrivateIdentifier:
			name = c.LiteralText()
			return true
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral,
			wrapperchecker.KindNumericLiteral:
			name = c.LiteralText()
			return true
		}
		// Anything else in the property-name slot is a
		// ComputedPropertyName wrapping an expression. Capture its
		// inner expression for the upstream-shaped lookup below — only
		// numeric literals and `Symbol.<name>` accesses produce a key
		// the rule tracks; string-literal computed names are ignored.
		if c.FirstChild() != nil {
			computedExpr = c.FirstChild()
			return true
		}
		return false
	})
	if name != "" {
		return name
	}
	if computedExpr == nil {
		return ""
	}
	return computedPropertyKey(computedExpr)
}

// computedPropertyKey mirrors upstream's getEsNodesForVariable
// computed-name branch: numeric literals use their text; `Symbol.X`
// accesses use a synthetic key derived from the property name; any
// other expression (string literal, arbitrary const, function call)
// returns "" so the rule treats the field as untrackable and skips it.
func computedPropertyKey(expr *wrapperchecker.Node) string {
	if expr == nil {
		return ""
	}
	if expr.Kind() == wrapperchecker.KindNumericLiteral {
		return expr.LiteralText()
	}
	if expr.Kind() == wrapperchecker.KindPropertyAccessExpression {
		recv := expr.PropertyAccessReceiver()
		if recv != nil && recv.Kind() == wrapperchecker.KindIdentifier &&
			recv.LiteralText() == "Symbol" {
			return fmt.Sprintf("\x00symbol:%s", expr.PropertyAccessName())
		}
	}
	return ""
}

// hasPrivateIdentifierName reports whether the property declaration's
// name is a `#`-private identifier (ECMAScript private fields).
// These are implicitly private without the `private` modifier.
func hasPrivateIdentifierName(n *wrapperchecker.Node) bool {
	private := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindPrivateIdentifier {
			private = true
			return true
		}
		return false
	})
	return private
}

// classWritesToProperty reports whether any code inside the class
// (other than the constructor's initializing assignments) assigns
// to `this.<name>`. Tracks `const self = this` aliases so writes
// performed through such aliases count as `this.<name>` writes.
func classWritesToProperty(cls, field *wrapperchecker.Node, name string) bool {
	written := false
	var walk func(n *wrapperchecker.Node, inCtor bool, aliases map[string]bool)
	walk = func(n *wrapperchecker.Node, inCtor bool, aliases map[string]bool) {
		if written || n == nil {
			return
		}
		if n == field {
			return
		}
		if n.Kind() == wrapperchecker.KindConstructor {
			ctorAliases := map[string]bool{}
			collectThisAliases(n, ctorAliases)
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c, true, ctorAliases)
				return written
			})
			return
		}
		// Methods, getters, and setters open a new function scope
		// where `this` rebinds — collect per-scope aliases so writes
		// via type-asserted aliases (`const that = {} as this & {...}`)
		// flag correctly.
		switch n.Kind() {
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			methodAliases := map[string]bool{}
			collectThisAliases(n, methodAliases)
			if len(methodAliases) > 0 {
				n.ForEachChild(func(c *wrapperchecker.Node) bool {
					walk(c, false, methodAliases)
					return written
				})
				return
			}
		}
		if !inCtor && (isThisAssignmentTo(n, name) || isAliasAssignmentTo(n, name, aliases) ||
			isThisElementAssignmentTo(n, name) || isAliasElementAssignmentTo(n, name, aliases)) {
			written = true
			return
		}
		if !inCtor && isDestructuringWriteTo(n, name, aliases) {
			written = true
			return
		}
		if isThisDeleteOf(n, name) || isAliasDeleteOf(n, name, aliases) {
			written = true
			return
		}
		if isThisIncrementOf(n, name) || isAliasIncrementOf(n, name, aliases) {
			written = true
			return
		}
		// Cross a function-like boundary: writes inside a nested
		// closure can outlive the constructor, so the field can't be
		// declared `readonly` even if the closure runs synchronously
		// during construction.
		nextInCtor := inCtor
		nextAliases := aliases
		switch n.Kind() {
		case wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			nextInCtor = false
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			// A nested class introduces its own `this` binding —
			// writes inside it can't reach the outer class's field
			// of the same name.
			if n != cls {
				return
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c, nextInCtor, nextAliases)
			return written
		})
	}
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c, false, nil)
		return written
	})
	return written
}

// collectThisAliases walks the immediate body of a function-like
// node and records `const X = this` declarations so writes through
// X are credited to `this`. Also picks up type-asserted `this`
// shapes — `const X = {} as this & { ... }` and unions/intersections
// involving `this` or `typeof <Class>` — since writes through such
// aliases mutate the class instance at runtime.
func collectThisAliases(fn *wrapperchecker.Node, out map[string]bool) {
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindVariableStatement {
			n.ForEachChild(func(decllist *wrapperchecker.Node) bool {
				decllist.ForEachChild(func(decl *wrapperchecker.Node) bool {
					if decl.Kind() != wrapperchecker.KindVariableDeclaration {
						return false
					}
					init := decl.VariableDeclarationInitializer()
					if init == nil {
						return false
					}
					if init.Kind() == wrapperchecker.KindThisKeyword {
						if name := variableDeclName(decl); name != "" {
							out[name] = true
						}
						return false
					}
					// `const X = {} as this & { ... }` and similar
					// type-asserted shapes alias the class instance at
					// runtime — mutations through X land on `this`.
					if asTarget := typeAssertionAnnotation(init); asTarget != nil &&
						typeAnnotationReferencesThisOrEnclosingClass(asTarget) {
						if name := variableDeclName(decl); name != "" {
							out[name] = true
						}
					}
					return false
				})
				return false
			})
		}
		// Don't descend into nested function-likes — `this` rebinds.
		// Skip the rebind check for fn itself (the entry node), so the
		// caller can pass a Constructor / MethodDeclaration / etc.
		if n != fn {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindMethodDeclaration,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindGetAccessor,
				wrapperchecker.KindSetAccessor,
				wrapperchecker.KindConstructor:
				return
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(fn)
}

func variableDeclName(decl *wrapperchecker.Node) string {
	var name string
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// isAliasAssignmentTo mirrors isThisAssignmentTo but accepts an
// identifier receiver as long as it's listed in aliases.
// isDestructuringWriteTo reports whether n is a destructuring
// assignment whose left-hand pattern targets `this.<name>` or
// `<alias>.<name>`. Handles `({ value: this.value } = ...)`,
// `({ ...this.value } = ...)`, and `[this.value] = ...`.
func isDestructuringWriteTo(n *wrapperchecker.Node, name string, aliases map[string]bool) bool {
	if n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	if n.BinaryOperatorKind() != wrapperchecker.KindEqualsToken {
		return false
	}
	left := n.BinaryLeft()
	if left == nil {
		return false
	}
	switch left.Kind() {
	case wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindParenthesizedExpression:
	default:
		return false
	}
	return patternTargets(left, name, aliases)
}

// patternTargets walks a destructuring pattern (an array or object
// literal used as a pattern) and reports whether any leaf target is
// `this.<name>` or `<alias>.<name>`.
func patternTargets(p *wrapperchecker.Node, name string, aliases map[string]bool) bool {
	found := false
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if found || n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindPropertyAccessExpression && n.PropertyAccessName() == name {
			recv := n.PropertyAccessReceiver()
			if recv != nil && (recv.Kind() == wrapperchecker.KindThisKeyword ||
				(recv.Kind() == wrapperchecker.KindIdentifier && aliases[recv.LiteralText()])) {
				found = true
				return
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return found
		})
	}
	walk(p)
	return found
}

// isThisElementAssignmentTo / isAliasElementAssignmentTo handle
// element-access writes like `this['prop'] = 1`. The string-literal
// index is the only form that can target a private field by name.
func isThisElementAssignmentTo(n *wrapperchecker.Node, name string) bool {
	left := isAssignmentLeft(n)
	if left == nil || left.Kind() != wrapperchecker.KindElementAccessExpression {
		return false
	}
	if elementAccessLiteralName(left) != name {
		return false
	}
	recv := left.ElementAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindThisKeyword
}

func isAliasElementAssignmentTo(n *wrapperchecker.Node, name string, aliases map[string]bool) bool {
	if len(aliases) == 0 {
		return false
	}
	left := isAssignmentLeft(n)
	if left == nil || left.Kind() != wrapperchecker.KindElementAccessExpression {
		return false
	}
	if elementAccessLiteralName(left) != name {
		return false
	}
	recv := left.ElementAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && aliases[recv.LiteralText()]
}

// isAssignmentLeft returns the LHS of a binary expression with an
// assignment operator, or nil for non-assignments.
func isAssignmentLeft(n *wrapperchecker.Node) *wrapperchecker.Node {
	if n.Kind() != wrapperchecker.KindBinaryExpression {
		return nil
	}
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken, wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken, wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken, wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken, wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return n.BinaryLeft()
	}
	return nil
}

// elementAccessLiteralName returns the string-literal index of an
// element access expression (`a['name']` → "name"), or "" when the
// index isn't a string literal.
func elementAccessLiteralName(n *wrapperchecker.Node) string {
	idx := n.ElementAccessIndex()
	if idx == nil {
		return ""
	}
	switch idx.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return idx.LiteralText()
	}
	return ""
}

func isAliasAssignmentTo(n *wrapperchecker.Node, name string, aliases map[string]bool) bool {
	if len(aliases) == 0 || n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken, wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken, wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken, wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken, wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
	default:
		return false
	}
	left := n.BinaryLeft()
	if left == nil || left.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if left.PropertyAccessName() != name {
		return false
	}
	recv := left.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && aliases[recv.LiteralText()]
}

func isAliasDeleteOf(n *wrapperchecker.Node, name string, aliases map[string]bool) bool {
	if len(aliases) == 0 || n.Kind() != wrapperchecker.KindDeleteExpression {
		return false
	}
	target := n.FirstChild()
	if target == nil || target.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if target.PropertyAccessName() != name {
		return false
	}
	recv := target.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && aliases[recv.LiteralText()]
}

func isAliasIncrementOf(n *wrapperchecker.Node, name string, aliases map[string]bool) bool {
	if len(aliases) == 0 {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindPrefixUnaryExpression, wrapperchecker.KindPostfixUnaryExpression:
	default:
		return false
	}
	if op := n.PrefixUnaryOperator(); op != "++" && op != "--" {
		return false
	}
	target := n.FirstChild()
	if target == nil || target.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if target.PropertyAccessName() != name {
		return false
	}
	recv := target.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindIdentifier && aliases[recv.LiteralText()]
}

func isThisAssignmentTo(n *wrapperchecker.Node, name string) bool {
	if n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	op := n.BinaryOperatorKind()
	switch op {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
	default:
		return false
	}
	left := n.BinaryLeft()
	if left == nil || left.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if left.PropertyAccessName() != name {
		return false
	}
	recv := left.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindThisKeyword
}

// isThisIncrementOf reports whether n is a `++` or `--` (prefix or
// postfix) on `this.<name>`. These mutate the field just like `=`.
func isThisIncrementOf(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindPrefixUnaryExpression, wrapperchecker.KindPostfixUnaryExpression:
	default:
		return false
	}
	op := n.PrefixUnaryOperator()
	if op != "++" && op != "--" {
		return false
	}
	target := n.FirstChild()
	if target == nil || target.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if target.PropertyAccessName() != name {
		return false
	}
	recv := target.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindThisKeyword
}

func isThisDeleteOf(n *wrapperchecker.Node, name string) bool {
	if n.Kind() != wrapperchecker.KindDeleteExpression {
		return false
	}
	target := n.FirstChild()
	if target == nil || target.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if target.PropertyAccessName() != name {
		return false
	}
	recv := target.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindThisKeyword
}

// typeAssertionAnnotation returns the annotation of an `expr as T` or
// `<T>expr` form, peeling parentheses. Nil for non-assertion shapes.
func typeAssertionAnnotation(expr *wrapperchecker.Node) *wrapperchecker.Node {
	for expr != nil && expr.Kind() == wrapperchecker.KindParenthesizedExpression {
		expr = expr.FirstChild()
	}
	if expr == nil {
		return nil
	}
	switch expr.Kind() {
	case wrapperchecker.KindAsExpression:
		return expr.AsExpressionTarget()
	case wrapperchecker.KindTypeAssertionExpression:
		return expr.TypeAssertionTarget()
	}
	return nil
}

// typeAnnotationReferencesThisOrEnclosingClass reports whether the
// type-node references `this` (e.g. `this`, `this & X`, `this | X`)
// or a `typeof <Class>` reference. The walk is structural — it
// doesn't resolve symbols. A typeof query in a `const X = {} as ...`
// declaration almost always refers to the enclosing class; the
// declaration shape is uncommon enough elsewhere that the heuristic
// is acceptable.
func typeAnnotationReferencesThisOrEnclosingClass(annot *wrapperchecker.Node) bool {
	if annot == nil {
		return false
	}
	found := false
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if found || n == nil {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindThisType,
			wrapperchecker.KindThisKeyword,
			wrapperchecker.KindTypeQuery:
			found = true
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return found
		})
	}
	walk(annot)
	return found
}
