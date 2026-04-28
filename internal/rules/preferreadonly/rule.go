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
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.HasPrivateModifier() {
		return
	}
	if n.HasReadonlyModifier() {
		return
	}
	if n.HasStaticModifier() {
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
	if classWritesToProperty(cls, n, name) {
		return
	}
	ctx.Report(n, "private field is never reassigned; declare it `readonly`")
}

func propertyName(n *wrapperchecker.Node) string {
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
		if !inCtor && (isThisAssignmentTo(n, name) || isAliasAssignmentTo(n, name, aliases)) {
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
			wrapperchecker.KindMethodDeclaration:
			nextInCtor = false
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
