// Package usewhile implements use-while: `for(;cond;)` and
// `for(;;)` (with body relying on break) are `while`-shaped — prefer
// the keyword that matches the meaning.
package usewhile

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-while"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindForStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Find the header substring between `for (` and `)`. Strip
	// comments to make slot detection robust.
	src := n.SourceText()
	_, rest, ok := strings.Cut(src, "(")
	if !ok {
		return
	}
	hdr, _, ok := strings.Cut(rest, ")")
	if !ok {
		return
	}
	hdr = stripComments(hdr)
	// Header has three slots separated by `;`. If init and update are
	// empty, the for is just a while.
	parts := strings.Split(hdr, ";")
	if len(parts) != 3 {
		return
	}
	init := strings.TrimSpace(parts[0])
	cond := strings.TrimSpace(parts[1])
	update := strings.TrimSpace(parts[2])
	if init == "" && update == "" && cond != "" {
		ctx.Report(n, "`for(;cond;)` is `while(cond)` — use `while`")
	}
}

func stripComments(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < len(s) {
				i += 2
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
