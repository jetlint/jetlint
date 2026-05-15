// Package nounusedprivateclassmembers implements the
// no-unused-private-class-members rule: a `#name` field declared in a
// class body but never read (or, for methods/accessors, never
// referenced) is almost always dead code. The rule scans each class
// body, collects every private declaration, and reports those whose
// only references — if any — are pure writes.
package nounusedprivateclassmembers

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unused-private-class-members"

// New constructs a nounusedprivateclassmembers rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: visit,
		wrapperchecker.KindClassExpression:  visit,
	}
}

type memberKind int

const (
	memberProperty memberKind = iota
	memberMethod
)

type member struct {
	name     string
	kind     memberKind
	decl     *wrapperchecker.Node
	readSeen bool
	refSeen  bool
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	members := collectMembers(n)
	if len(members) == 0 {
		return
	}
	classifyReferences(n, members)
	for _, m := range members {
		if m.kind == memberMethod {
			if !m.refSeen {
				ctx.Report(m.decl, "'"+m.name+"' is defined but never used.")
			}
			continue
		}
		if !m.readSeen {
			ctx.Report(m.decl, "'"+m.name+"' is defined but never used.")
		}
	}
}

func collectMembers(class *wrapperchecker.Node) []*member {
	var out []*member
	class.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPropertyDeclaration:
			if name := privateNameOf(c); name != nil {
				out = append(out, &member{
					name: name.LiteralText(),
					kind: memberProperty,
					decl: name,
				})
			}
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			if name := privateNameOf(c); name != nil {
				out = append(out, &member{
					name: name.LiteralText(),
					kind: memberMethod,
					decl: name,
				})
			}
		}
		return false
	})
	// Collapse paired getter/setter declarations with the same name.
	seen := map[string]*member{}
	dedup := out[:0]
	for _, m := range out {
		if prev, ok := seen[m.name]; ok && m.kind == memberMethod && prev.kind == memberMethod {
			continue
		}
		seen[m.name] = m
		dedup = append(dedup, m)
	}
	return dedup
}

func privateNameOf(decl *wrapperchecker.Node) *wrapperchecker.Node {
	var name *wrapperchecker.Node
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindPrivateIdentifier {
			name = c
			return true
		}
		return false
	})
	return name
}

// classifyReferences walks the class subtree (without descending into
// nested classes, which have their own private-name scope) and updates
// each member's flags based on how its name is used.
func classifyReferences(class *wrapperchecker.Node, members []*member) {
	byName := map[string]*member{}
	for _, m := range members {
		byName[m.name] = m
	}
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindClassDeclaration,
				wrapperchecker.KindClassExpression:
				return false
			case wrapperchecker.KindPrivateIdentifier:
				if m, ok := byName[c.LiteralText()]; ok && c.Pos() != m.decl.Pos() {
					m.refSeen = true
					if isReadReference(c) {
						m.readSeen = true
					}
				}
			}
			walk(c)
			return false
		})
	}
	walk(class)
}

// isReadReference returns whether a PrivateIdentifier reference reads
// the field's value. A PrivateIdentifier appears either as the name
// token of a PropertyAccessExpression (`this.#x`) or as the left
// operand of `in` (`#x in obj`); the latter is always treated as a
// read.
func isReadReference(name *wrapperchecker.Node) bool {
	access := name.Parent()
	if access == nil || access.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return true
	}
	if isDestructuringWriteTarget(access) {
		return false
	}
	return isValueConsumed(access)
}

// isDestructuringWriteTarget returns true when `access` is a write
// target of a destructuring assignment (`[this.#x] = bar`,
// `({a: this.#x} = bar)`, `[...this.#x] = bar`, …). A computed key
// (`{[this.#x]: a} = bar`) is *not* a write target — the key
// expression is evaluated to look up the destination, so it counts as
// a read.
func isDestructuringWriteTarget(access *wrapperchecker.Node) bool {
	crossedComputedKey := false
	curr := access
	parent := skipTransparent(curr.Parent())
	for parent != nil {
		switch parent.Kind() {
		case wrapperchecker.KindBinaryExpression:
			op := parent.BinaryOperatorKind()
			if op != wrapperchecker.KindEqualsToken {
				return false
			}
			left := skipTransparent(parent.BinaryLeft())
			if !isPositionMatch(left, curr) {
				return false
			}
			return !crossedComputedKey
		case wrapperchecker.KindComputedPropertyName:
			crossedComputedKey = true
		case wrapperchecker.KindArrayLiteralExpression,
			wrapperchecker.KindObjectLiteralExpression,
			wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindSpreadElement,
			wrapperchecker.KindSpreadAssignment:
			// Destructuring-pattern wrappers — continue walking up.
		default:
			return false
		}
		curr = parent
		parent = skipTransparent(curr.Parent())
	}
	return false
}

// isValueConsumed walks the parent chain to determine whether the
// expression `curr`'s value is read anywhere. Parens / TS-only type
// wrappers are transparent. For non-destructuring contexts, this
// mirrors oxc's `is_value_context` recursion through conditionals,
// logical operators, unary updates, etc.
func isValueConsumed(curr *wrapperchecker.Node) bool {
	parent := skipTransparent(curr.Parent())
	for parent != nil {
		switch parent.Kind() {
		case wrapperchecker.KindBinaryExpression:
			op := parent.BinaryOperatorKind()
			if isAssignmentOperator(op) {
				right := skipTransparent(parent.BinaryRight())
				if isPositionMatch(right, curr) {
					return true
				}
				if op == wrapperchecker.KindEqualsToken {
					return false
				}
				// Compound assignment LHS read+write — recurse to
				// see whether the assignment's result is consumed.
			} else if isLogicalOperator(op) {
				left := skipTransparent(parent.BinaryLeft())
				if isPositionMatch(left, curr) {
					return true
				}
				// Right operand of `&&` / `||` / `??` — recurse.
			} else {
				return true
			}
		case wrapperchecker.KindConditionalExpression:
			if isPositionMatch(parent.ConditionalCondition(), curr) {
				return true
			}
			// Consequent or alternate — recurse.
		case wrapperchecker.KindPrefixUnaryExpression,
			wrapperchecker.KindPostfixUnaryExpression:
			// `++` / `--` — recurse to see if the value is consumed.
		case wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindFunctionDeclaration:
			body := parent.FunctionBody()
			if body != nil && body.Kind() != wrapperchecker.KindBlock && isPositionMatch(body, curr) {
				return true
			}
			return false
		case wrapperchecker.KindParameter:
			init := parent.ParameterInitializer()
			return isPositionMatch(init, curr)
		case wrapperchecker.KindBindingElement:
			init := parent.BindingElementInitializer()
			return isPositionMatch(init, curr)
		case wrapperchecker.KindForInStatement, wrapperchecker.KindForOfStatement:
			return isPositionMatch(parent.ForInOrOfExpression(), curr)
		case wrapperchecker.KindExpressionStatement:
			return false
		case wrapperchecker.KindReturnStatement,
			wrapperchecker.KindCallExpression,
			wrapperchecker.KindNewExpression,
			wrapperchecker.KindVariableDeclaration,
			wrapperchecker.KindPropertyDeclaration,
			wrapperchecker.KindArrayLiteralExpression,
			wrapperchecker.KindObjectLiteralExpression,
			wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindElementAccessExpression,
			wrapperchecker.KindPropertyAccessExpression,
			wrapperchecker.KindTemplateExpression,
			wrapperchecker.KindTemplateSpan,
			wrapperchecker.KindTaggedTemplateExpression,
			wrapperchecker.KindComputedPropertyName,
			wrapperchecker.KindIfStatement,
			wrapperchecker.KindSpreadElement,
			wrapperchecker.KindSpreadAssignment,
			wrapperchecker.KindSwitchStatement,
			wrapperchecker.KindCaseClause,
			wrapperchecker.KindThrowStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindAwaitExpression,
			wrapperchecker.KindYieldExpression,
			wrapperchecker.KindTypeOfExpression,
			wrapperchecker.KindVoidExpression,
			wrapperchecker.KindDeleteExpression:
			return true
		default:
			return false
		}
		curr = parent
		parent = skipTransparent(curr.Parent())
	}
	return false
}

// skipTransparent walks upward past parens and TS-only type wrappers,
// which oxc ignores when classifying value contexts.
func skipTransparent(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil {
		switch n.Kind() {
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindAsExpression,
			wrapperchecker.KindSatisfiesExpression,
			wrapperchecker.KindNonNullExpression,
			wrapperchecker.KindTypeAssertionExpression:
			n = n.Parent()
			continue
		}
		return n
	}
	return n
}

// isPositionMatch returns whether two nodes refer to the same source
// span. The TS-go wrapper materialises Node values on demand, so the
// same logical AST position may produce distinct *Node pointers.
func isPositionMatch(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Pos() == b.Pos() && a.End() == b.End()
}

func isAssignmentOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

func isLogicalOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken,
		wrapperchecker.KindQuestionQuestionToken:
		return true
	}
	return false
}
