// Package nounsafeargument implements the no-unsafe-argument rule:
// flag `f(x)` where x has type any but the parameter expects something
// more specific.
package nounsafeargument

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-argument"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:           visit,
		wrapperchecker.KindNewExpression:            visit,
		wrapperchecker.KindTaggedTemplateExpression: visitTagged,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	args := n.CallArguments()
	for i, arg := range args {
		if arg.Kind() == wrapperchecker.KindSpreadElement {
			checkSpread(ctx, n, arg, i)
			continue
		}
		argT := ctx.TypeOf(arg)
		if argT == nil {
			continue
		}
		paramT := ctx.Checker().ContextualTypeForArgument(n, i)
		if paramT == nil {
			continue
		}
		if paramT.IsAny() || paramT.IsUnknown() {
			continue
		}
		// Top-level `any` always flags. Nested `any` inside a generic
		// type only flags when the user wrote `any` explicitly — bare
		// `new Map()` defaults to `Map<any, any>` in tsgo, but should
		// have been narrowed contextually to the parameter's shape.
		if argT.IsAny() || (argHasExplicitAnyKeyword(arg) && argIsUnsafe(argT, paramT, 8)) {
			ctx.Report(arg, "passing an `any` value to a parameter with a more specific type")
		}
	}
}

func checkSpread(ctx *engine.Context, call *wrapperchecker.Node, spread *wrapperchecker.Node, baseIdx int) {
	expr := spread.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	// `...x` where x is `any` (or `any[]`): each downstream parameter
	// effectively receives an `any`. Flag once if any param is more
	// specific than any.
	if t.IsAny() || elementTypeIsAny(t) {
		// Check at least one downstream non-any parameter exists.
		paramT := ctx.Checker().ContextualTypeForArgument(call, baseIdx)
		if paramT != nil && !paramT.IsAny() && !paramT.IsUnknown() {
			ctx.Report(spread, "spreading an `any` value into parameters with more specific types")
			return
		}
	}
	// Tuple spread: walk element types against parameter slots.
	if t.IsTupleType() {
		args := t.TypeArguments()
		for i, elemT := range args {
			paramT := ctx.Checker().ContextualTypeForArgument(call, baseIdx+i)
			if paramT == nil {
				continue
			}
			if paramT.IsAny() || paramT.IsUnknown() {
				continue
			}
			if argIsUnsafe(elemT, paramT, 8) {
				ctx.Report(spread, "passing an `any` value through a spread to a parameter with a more specific type")
				return
			}
		}
	}
}

// argHasExplicitAnyKeyword reports whether the AST of an argument
// expression contains a literal `any` type-keyword node — i.e. the
// user wrote `<any>` somewhere. Used to distinguish between explicit
// `any` (which should flag through generic type arguments) and
// default-inferred any from `new Map()` etc.
func argHasExplicitAnyKeyword(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindAnyKeyword {
		return true
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if argHasExplicitAnyKeyword(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

func elementTypeIsAny(t *wrapperchecker.Type) bool {
	if elem := t.ArrayElementType(); elem != nil {
		return elem.IsAny()
	}
	return false
}

// argIsUnsafe reports whether passing actual to a slot expecting
// declared smuggles `any` through the type checker. Recurses into
// matching generic type arguments (Set<any> vs Set<string>).
func argIsUnsafe(actual, declared *wrapperchecker.Type, depth int) bool {
	if actual == nil || depth <= 0 {
		return false
	}
	if actual.IsAny() {
		if declared != nil && (declared.IsAny() || declared.IsUnknown()) {
			return false
		}
		return true
	}
	if actual.IsUnion() {
		for _, m := range actual.UnionMembers() {
			if argIsUnsafe(m, declared, depth-1) {
				return true
			}
		}
		return false
	}
	if declared == nil {
		return false
	}
	actualArgs := actual.TypeArguments()
	declaredArgs := declared.TypeArguments()
	if len(actualArgs) > 0 && len(actualArgs) == len(declaredArgs) &&
		actual.SymbolName() == declared.SymbolName() && actual.SymbolName() != "" {
		for i := range actualArgs {
			if argIsUnsafe(actualArgs[i], declaredArgs[i], depth-1) {
				return true
			}
		}
	}
	return false
}

// visitTagged handles `tag` template `${a}${b}` — each interpolation is
// an argument to the tag function, with parameter slot offset by one
// (the first parameter is the TemplateStringsArray).
func visitTagged(ctx *engine.Context, n *wrapperchecker.Node) {
	args := n.TaggedTemplateInterpolations()
	for i, arg := range args {
		argT := ctx.TypeOf(arg)
		if argT == nil || !argT.IsAny() {
			continue
		}
		paramT := ctx.Checker().ContextualTypeForArgument(n, i+1)
		if paramT == nil {
			continue
		}
		if paramT.IsAny() || paramT.IsUnknown() {
			continue
		}
		ctx.Report(arg, "passing an `any` value to a parameter with a more specific type")
	}
}
