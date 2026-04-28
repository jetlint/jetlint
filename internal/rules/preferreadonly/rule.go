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
// to `this.<name>`.
func classWritesToProperty(cls, field *wrapperchecker.Node, name string) bool {
	written := false
	var walk func(n *wrapperchecker.Node, inCtor bool)
	walk = func(n *wrapperchecker.Node, inCtor bool) {
		if written || n == nil {
			return
		}
		if n == field {
			return
		}
		if n.Kind() == wrapperchecker.KindConstructor {
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c, true)
				return written
			})
			return
		}
		if !inCtor && isThisAssignmentTo(n, name) {
			written = true
			return
		}
		if isThisDeleteOf(n, name) {
			written = true
			return
		}
		if isThisIncrementOf(n, name) {
			written = true
			return
		}
		// Cross a function-like boundary: writes inside a nested
		// closure can outlive the constructor, so the field can't be
		// declared `readonly` even if the closure runs synchronously
		// during construction.
		nextInCtor := inCtor
		switch n.Kind() {
		case wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration:
			nextInCtor = false
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c, nextInCtor)
			return written
		})
	}
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c, false)
		return written
	})
	return written
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
