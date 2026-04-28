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
		if isThisIncrementOf(n, name) {
			written = true
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c, inCtor)
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
		wrapperchecker.KindPlusEqualsToken:
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

func isThisIncrementOf(n *wrapperchecker.Node, name string) bool {
	if n.Kind() != wrapperchecker.KindPrefixUnaryExpression &&
		n.Kind() != wrapperchecker.KindPostfixUnaryExpression {
		return false
	}
	op := n.PrefixUnaryOperator()
	if op != "++" && op != "--" {
		return false
	}
	operand := n.FirstChild()
	if operand == nil || operand.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if operand.PropertyAccessName() != name {
		return false
	}
	recv := operand.PropertyAccessReceiver()
	return recv != nil && recv.Kind() == wrapperchecker.KindThisKeyword
}
