// Package preferregexpexec implements the prefer-regexp-exec rule:
// flag `s.match(regex)` where the regex's `exec` method on the string
// receiver would be the better shape (no global flag, string receiver).
package preferregexpexec

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-regexp-exec"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "match" {
		return
	}
	args := n.CallArguments()
	if len(args) != 1 {
		return
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !isPureString(rt) {
		return
	}
	arg := args[0]
	if !argIsNonGlobalRegExpOrString(ctx, arg) {
		return
	}
	ctx.Report(n, "prefer the regex's match-all shape over string.match — it's faster and avoids the global-flag iteration footgun")
}

// isPureString reports whether t is exactly string (or a union where
// every member is string). Excludes string-or-something-else unions
// to match upstream's handling of `string | string[]` and similar.
// Branded intersections like `string & { __brand: void }` are still
// treated as strings — they're a stylistic flavor of the primitive.
func isPureString(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isPureString(m) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isPureString(m) {
				return true
			}
		}
		return false
	}
	return t.IsStringLike() && !t.IsAny()
}

// argIsNonGlobalRegExpOrString reports whether the argument can be
// safely rewritten as `regex.exec(s)`. Literals and known RegExp
// constructions are inspected for the `g` flag (which changes match
// semantics and disqualifies the rewrite). Identifiers are traced to
// their initializers when those exist on a single-declaration
// variable. Invalid pattern strings (e.g. `'[a-z'`) are rejected
// since the autofix would produce a runtime error.
func argIsNonGlobalRegExpOrString(ctx *engine.Context, arg *wrapperchecker.Node) bool {
	hasG, known := regexExprHasGlobalFlag(ctx, arg, 4)
	if known {
		return !hasG
	}
	switch arg.Kind() {
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return looksLikeValidRegexPattern(arg.LiteralText())
	}
	t := ctx.TypeOf(arg)
	if t == nil {
		return false
	}
	if t.IsStringLike() {
		return true
	}
	return false
}

// regexExprHasGlobalFlag inspects an expression that produces a
// RegExp value. Returns (hasG, known=true) when the flag set can be
// determined from one of: a regex literal, a `new RegExp(p, flags)`
// or `RegExp(p, flags)` call with a literal flags string, or an
// identifier whose lone initializer matches one of the above.
// Returns known=false otherwise.
func regexExprHasGlobalFlag(ctx *engine.Context, n *wrapperchecker.Node, depth int) (bool, bool) {
	if n == nil || depth <= 0 {
		return false, false
	}
	switch n.Kind() {
	case wrapperchecker.KindRegularExpressionLiteral:
		text := n.LiteralText()
		if idx := strings.LastIndexByte(text, '/'); idx >= 0 {
			return strings.Contains(text[idx+1:], "g"), true
		}
		return false, true
	case wrapperchecker.KindNewExpression, wrapperchecker.KindCallExpression:
		callee := n.CalleeExpression()
		if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
			return false, false
		}
		if callee.LiteralText() != "RegExp" {
			return false, false
		}
		args := n.CallArguments()
		if len(args) < 2 {
			return false, true
		}
		flagsArg := args[1]
		switch flagsArg.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return strings.Contains(flagsArg.LiteralText(), "g"), true
		case wrapperchecker.KindIdentifier:
			if flagsArg.LiteralText() == "undefined" {
				return false, true
			}
		}
		return false, false
	case wrapperchecker.KindIdentifier:
		init := identifierSoleInitializer(ctx, n)
		if init == nil {
			return false, false
		}
		return regexExprHasGlobalFlag(ctx, init, depth-1)
	}
	return false, false
}

// identifierSoleInitializer returns the initializer of the variable
// declaration that uniquely defines `id`, or nil when the symbol has
// no declaration, multiple declarations, or no initializer (typed
// parameters, function-scoped lets without init, etc.).
func identifierSoleInitializer(ctx *engine.Context, id *wrapperchecker.Node) *wrapperchecker.Node {
	sym := ctx.Checker().SymbolOf(id)
	if sym == nil {
		return nil
	}
	decls := sym.Declarations()
	if len(decls) != 1 {
		return nil
	}
	return decls[0].VariableDeclarationInitializer()
}

// looksLikeValidRegexPattern is a quick gate against obviously
// malformed regex patterns expressed as strings. Doesn't aim to be a
// full regex parser — only catches unmatched character classes and
// groups, which is the autofix-breaking shape upstream guards against.
func looksLikeValidRegexPattern(s string) bool {
	depthGroup := 0
	inClass := false
	escape := false
	for _, c := range s {
		if escape {
			escape = false
			continue
		}
		switch c {
		case '\\':
			escape = true
		case '[':
			if !inClass {
				inClass = true
			}
		case ']':
			if inClass {
				inClass = false
			}
		case '(':
			if !inClass {
				depthGroup++
			}
		case ')':
			if !inClass {
				depthGroup--
				if depthGroup < 0 {
					return false
				}
			}
		}
	}
	return depthGroup == 0 && !inClass && !escape
}
