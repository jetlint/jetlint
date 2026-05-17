// Package noglobaleval implements no-global-eval: flag calls that
// route to the global script-evaluator. We accept:
//
//   - bare X(...) where X is the global identifier "eval"
//     (skipped when "eval" is locally shadowed),
//   - window.X(...), self.X(...), globalThis.X(...), global.X(...),
//   - computed forms window["X"], etc.,
//   - and indirect (0, EXPR)(...) forms of the above,
//   - plus locally-bound aliases like `var BIND = eval`.
//
// We intentionally do not flag `this.X(...)` because `this` is
// undefined in module scope and bound to a non-global in any
// enclosing function — disambiguating is not worth the false
// positives biome's own tests show.
package noglobaleval

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-global-eval"
const target = "eval"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	aliases := collectAliases(src)
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n.Kind() == wrapperchecker.KindCallExpression {
			callee := unwrapCommaIndirection(n.CalleeExpression())
			if isGlobalTargetCallee(callee, aliases) {
				if !shadowed(n) {
					ctx.Report(n, "global script-evaluator call executes arbitrary code — avoid it")
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c)
		return false
	})
}

// isGlobalTargetCallee returns true for bare `eval`, member access
// on a global object (window/self/globalThis/global), or an alias
// previously bound to one of those.
func isGlobalTargetCallee(callee *wrapperchecker.Node, aliases map[string]bool) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		t := callee.LiteralText()
		return t == target || aliases[t]
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName() == target && isGlobalReceiver(callee.PropertyAccessReceiver())
	case wrapperchecker.KindElementAccessExpression:
		arg := callee.ElementAccessIndex()
		if arg == nil || arg.Kind() != wrapperchecker.KindStringLiteral {
			return false
		}
		return arg.LiteralText() == target && isGlobalReceiver(callee.ElementAccessReceiver())
	}
	return false
}

// shadowed walks up the AST from a call site looking for an
// enclosing scope that declares `eval` as a parameter or local
// variable. Such a binding eclipses the global so the call should
// not be flagged.
func shadowed(call *wrapperchecker.Node) bool {
	for p := call.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindConstructor,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			if scopeShadows(p) {
				return true
			}
		case wrapperchecker.KindBlock,
			wrapperchecker.KindSourceFile:
			if scopeShadows(p) {
				return true
			}
		}
	}
	return false
}

// scopeShadows reports whether the scope-bearing node declares
// `eval` as a parameter, function name, or variable.
func scopeShadows(scope *wrapperchecker.Node) bool {
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindParameter:
			c.ForEachChild(func(p *wrapperchecker.Node) bool {
				if p.Kind() == wrapperchecker.KindIdentifier && p.LiteralText() == target {
					found = true
					return true
				}
				return false
			})
		case wrapperchecker.KindVariableStatement:
			c.ForEachChild(func(decl *wrapperchecker.Node) bool {
				if decl.Kind() != wrapperchecker.KindVariableDeclarationList {
					return false
				}
				decl.ForEachChild(func(vd *wrapperchecker.Node) bool {
					if vd.Kind() != wrapperchecker.KindVariableDeclaration {
						return false
					}
					// First identifier child is the binding name;
					// later children are type annotation and
					// initializer expression — don't conflate.
					var first *wrapperchecker.Node
					vd.ForEachChild(func(n *wrapperchecker.Node) bool {
						first = n
						return true
					})
					if first != nil && first.Kind() == wrapperchecker.KindIdentifier && first.LiteralText() == target {
						found = true
					}
					return found
				})
				return found
			})
		}
		return found
	})
	return found
}

func isGlobalReceiver(recv *wrapperchecker.Node) bool {
	if recv == nil {
		return false
	}
	switch recv.Kind() {
	case wrapperchecker.KindIdentifier:
		switch recv.LiteralText() {
		case "window", "globalThis", "self", "global":
			return true
		}
	case wrapperchecker.KindPropertyAccessExpression:
		name := recv.PropertyAccessName()
		if name == "window" || name == "globalThis" || name == "self" || name == "global" {
			return isGlobalReceiver(recv.PropertyAccessReceiver())
		}
	case wrapperchecker.KindElementAccessExpression:
		arg := recv.ElementAccessIndex()
		if arg != nil && arg.Kind() == wrapperchecker.KindStringLiteral {
			t := arg.LiteralText()
			if t == "window" || t == "globalThis" || t == "self" || t == "global" {
				return isGlobalReceiver(recv.ElementAccessReceiver())
			}
		}
	}
	return false
}

func collectAliases(src *wrapperchecker.Node) map[string]bool {
	out := map[string]bool{}
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n.Kind() == wrapperchecker.KindVariableDeclaration {
			var name string
			var init *wrapperchecker.Node
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if c.Kind() == wrapperchecker.KindIdentifier {
					if name == "" {
						name = c.LiteralText()
					}
				} else if !isTypeAnnotation(c) {
					init = c
				}
				return false
			})
			if name != "" && init != nil && isAliasInitializer(init) {
				out[name] = true
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c)
		return false
	})
	return out
}

func isAliasInitializer(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == target
	case wrapperchecker.KindPropertyAccessExpression:
		return n.PropertyAccessName() == target && isGlobalReceiver(n.PropertyAccessReceiver())
	case wrapperchecker.KindElementAccessExpression:
		arg := n.ElementAccessIndex()
		if arg == nil || arg.Kind() != wrapperchecker.KindStringLiteral {
			return false
		}
		return arg.LiteralText() == target && isGlobalReceiver(n.ElementAccessReceiver())
	}
	return false
}

func isTypeAnnotation(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindTypeReference,
		wrapperchecker.KindTypeLiteral,
		wrapperchecker.KindUnionType,
		wrapperchecker.KindIntersectionType,
		wrapperchecker.KindFunctionType,
		wrapperchecker.KindConstructorType,
		wrapperchecker.KindArrayType,
		wrapperchecker.KindTupleType:
		return true
	}
	return false
}

func unwrapCommaIndirection(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		n = inner
	}
	if n != nil && n.Kind() == wrapperchecker.KindBinaryExpression &&
		n.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
		return n.BinaryRight()
	}
	return n
}
