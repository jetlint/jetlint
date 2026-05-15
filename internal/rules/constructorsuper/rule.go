// Package constructorsuper implements the constructor-super rule:
// derived-class constructors must call super(); non-derived
// constructors must not; and the super class must be a constructable
// value (not `null`, a literal, or the result of certain operators).
//
// TypeScript already flags many of these at compile time, but the
// rule ships for ESLint parity and to catch errors in .js files
// linted alongside.
package constructorsuper

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/astflow"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "constructor-super"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: r.visit,
		wrapperchecker.KindClassExpression:  r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, class *wrapperchecker.Node) {
	ctor := findConstructor(class)
	if ctor == nil {
		return
	}
	if !hasBody(ctor) {
		return
	}
	parent := extendsExpression(class)
	parentKind := classifyParent(parent)
	hasSuper := astflow.ConstructorHasSuperCall(ctor)
	switch parentKind {
	case parentNone:
		if hasSuper {
			ctx.Report(ctor, "constructor of non-derived class must not call super()")
		}
	case parentUnconstructable:
		if hasSuper {
			ctx.Report(ctor, "super() is not allowed when the superclass is not a constructor")
		}
	case parentNull:
		if hasSuper {
			ctx.Report(ctor, "super() is not allowed when the superclass is null")
			return
		}
		if !constructorHasReturnWithValue(ctor) {
			ctx.Report(ctor, "constructor extending null must return an explicit value")
		}
	case parentConstructable:
		status := astflow.ConstructorSuperCallStatus(ctor)
		switch status {
		case astflow.SuperNone:
			ctx.Report(ctor, "constructor of a derived class must call super()")
		case astflow.SuperSome:
			ctx.Report(ctor, "constructor of a derived class must call super() on every path")
		case astflow.SuperMultiple:
			ctx.Report(ctor, "constructor of a derived class must not call super() more than once")
		}
	}
}

// constructorHasReturnWithValue reports whether a constructor body
// contains a `return <expr>` statement on any path (matches oxc's
// has_return_with_value helper).
func constructorHasReturnWithValue(ctor *wrapperchecker.Node) bool {
	var body *wrapperchecker.Node
	ctor.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
			return true
		}
		return false
	})
	if body == nil {
		return false
	}
	return statementsHaveReturnWithValue(body)
}

func statementsHaveReturnWithValue(stmt *wrapperchecker.Node) bool {
	if stmt == nil {
		return false
	}
	switch stmt.Kind() {
	case wrapperchecker.KindReturnStatement:
		var arg *wrapperchecker.Node
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			arg = c
			return true
		})
		return arg != nil
	case wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression:
		return false
	}
	found := false
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if statementsHaveReturnWithValue(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

// parentKind classifies the value of the `extends` clause. Matches
// oxc's SuperClassType (None / Null / Invalid / Valid).
type parentKind int

const (
	// parentNone: class has no extends clause.
	parentNone parentKind = iota
	// parentConstructable: extends value can plausibly be a
	// constructor — identifier, member access, `this`, call result,
	// `new` expression, parenthesized constructable, etc.
	parentConstructable
	// parentUnconstructable: extends value is known not to be a
	// constructor — primitive literal, arithmetic/bitwise binary,
	// certain compound assignments.
	parentUnconstructable
	// parentNull: extends value is the `null` literal.
	parentNull
)

func classifyParent(n *wrapperchecker.Node) parentKind {
	if n == nil {
		return parentNone
	}
	n = stripParens(n)
	if n == nil {
		return parentNone
	}
	if n.Kind() == wrapperchecker.KindNullKeyword {
		return parentNull
	}
	if isInvalidSuperClass(n) {
		return parentUnconstructable
	}
	return parentConstructable
}

// isInvalidSuperClass mirrors oxc's logic: only certain literal /
// binary / compound-assignment shapes are considered unconstructable.
// Identifiers, calls, `new`, member access, arrow/function
// expressions, etc. are treated as potentially constructable
// (oxlint takes the optimistic stance for unknown expressions).
func isInvalidSuperClass(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	n = stripParens(n)
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindTemplateExpression,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword:
		return true
	case wrapperchecker.KindConditionalExpression:
		whenTrue, whenFalse := n.ConditionalBranches()
		return isInvalidSuperClass(whenTrue) && isInvalidSuperClass(whenFalse)
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		switch op {
		case wrapperchecker.KindEqualsToken,
			wrapperchecker.KindAmpersandAmpersandEqualsToken:
			return isInvalidSuperClass(n.BinaryRight())
		case wrapperchecker.KindBarBarEqualsToken,
			wrapperchecker.KindQuestionQuestionEqualsToken:
			// `A ||= B` / `A ??= B`: A may already be truthy /
			// non-nullish — value is then A (unknown), so the result
			// could be constructable. Don't flag.
			return false
		case wrapperchecker.KindAmpersandAmpersandToken:
			// `A && B`: if A truthy, result is B; otherwise A
			// (falsy). Invalid only if B is invalid.
			return isInvalidSuperClass(n.BinaryRight())
		case wrapperchecker.KindBarBarToken,
			wrapperchecker.KindQuestionQuestionToken:
			// `A || B` / `A ?? B`: either A or B; could be valid if
			// A is valid. Don't flag.
			return false
		case wrapperchecker.KindCommaToken:
			// `(A, B)`: result is B.
			return isInvalidSuperClass(n.BinaryRight())
		}
		// Arithmetic, bitwise, comparison: numeric/boolean result.
		return true
	}
	return false
}

// findConstructor returns the Constructor method of a class, if any.
func findConstructor(class *wrapperchecker.Node) *wrapperchecker.Node {
	var ctor *wrapperchecker.Node
	class.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindConstructor {
			ctor = c
			return true
		}
		return false
	})
	return ctor
}

// extendsExpression returns the expression of the class' `extends`
// heritage clause, or nil if there is none. The TS AST flattens
// heritage entries; we take the first ExpressionWithTypeArguments
// under the ExtendsKeyword clause.
func extendsExpression(class *wrapperchecker.Node) *wrapperchecker.Node {
	var expr *wrapperchecker.Node
	class.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindHeritageClause {
			return false
		}
		if c.HeritageClauseToken() != wrapperchecker.KindExtendsKeyword {
			return false
		}
		c.ForEachChild(func(entry *wrapperchecker.Node) bool {
			if entry.Kind() != wrapperchecker.KindExpressionWithTypeArguments {
				return false
			}
			entry.ForEachChild(func(child *wrapperchecker.Node) bool {
				if expr == nil {
					expr = child
				}
				return true
			})
			return true
		})
		return true
	})
	return expr
}

// hasBody reports whether a Constructor has a block body (an
// abstract or overload-only constructor has no body).
func hasBody(ctor *wrapperchecker.Node) bool {
	has := false
	ctor.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			has = true
			return true
		}
		return false
	})
	return has
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
