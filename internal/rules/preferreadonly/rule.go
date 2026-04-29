// Package preferreadonly implements the prefer-readonly rule: flag
// private class fields that are never reassigned after construction.
package preferreadonly

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-readonly"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyDeclaration: visit,
		wrapperchecker.KindParameter:           visitParam,
	}
}

// visitParam handles constructor parameter properties — `constructor
// (private foo = 1) {}` synthesizes a class field with the same
// modifiers and is subject to the same readonly check.
func visitParam(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.HasPrivateModifier() {
		return
	}
	if n.HasReadonlyModifier() {
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

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
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
// assigns to `ClassName.<name>` outside the field declaration itself.
// Walks every nested expression but stops at the field declaration so
// the initializer doesn't count as a reassignment.
func classWritesToStatic(cls, field *wrapperchecker.Node, name, clsName string) bool {
	written := false
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if written || n == nil || n == field {
			return
		}
		if isStaticAssignmentTo(n, name, clsName) ||
			isStaticIncrementOf(n, name, clsName) ||
			isStaticDeleteOf(n, name, clsName) {
			written = true
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return written
		})
	}
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c)
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
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if name != "" {
			return true
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier, wrapperchecker.KindPrivateIdentifier:
			name = c.LiteralText()
			return true
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
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
// X are credited to `this`.
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
					if init == nil || init.Kind() != wrapperchecker.KindThisKeyword {
						return false
					}
					if name := variableDeclName(decl); name != "" {
						out[name] = true
					}
					return false
				})
				return false
			})
		}
		// Don't descend into nested function-likes — `this` rebinds.
		switch n.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression, wrapperchecker.KindMethodDeclaration:
			return
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
