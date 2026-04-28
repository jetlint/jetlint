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
	if !argIsNonGlobalRegExpOrString(arg) {
		return
	}
	ctx.Report(n, "prefer the regex's match-all shape over string.match — it's faster and avoids the global-flag iteration footgun")
}

// isPureString reports whether t is exactly string (or a union where
// every member is string). Excludes string-or-something-else unions
// to match upstream's handling of `string | string[]` and similar.
func isPureString(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isPureString(m) {
				return false
			}
		}
		return true
	}
	return t.IsStringLike() && !t.IsAny()
}

// argIsNonGlobalRegExpOrString reports whether the argument is a
// regex literal without the global flag, or a string literal.
func argIsNonGlobalRegExpOrString(arg *wrapperchecker.Node) bool {
	switch arg.Kind() {
	case wrapperchecker.KindRegularExpressionLiteral:
		text := arg.LiteralText()
		if idx := strings.LastIndexByte(text, '/'); idx >= 0 {
			flags := text[idx+1:]
			if strings.Contains(flags, "g") {
				return false
			}
		}
		return true
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return true
	}
	return false
}
