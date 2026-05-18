// Package usenodejsimportprotocol implements use-nodejs-import-protocol:
// `import "fs"` resolves to either the built-in or a user package
// named "fs". `import "node:fs"` is unambiguous.
package usenodejsimportprotocol

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-nodejs-import-protocol"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visit,
		wrapperchecker.KindCallExpression:    visitRequire,
	}
}

// Node built-in module names (Node 20+).
var nodeBuiltins = map[string]bool{
	"assert": true, "async_hooks": true, "buffer": true, "child_process": true,
	"cluster": true, "console": true, "constants": true, "crypto": true,
	"dgram": true, "diagnostics_channel": true, "dns": true, "domain": true,
	"events": true, "fs": true, "fs/promises": true, "http": true, "http2": true,
	"https": true, "inspector": true, "module": true, "net": true, "os": true,
	"path": true, "path/posix": true, "path/win32": true, "perf_hooks": true,
	"process": true, "punycode": true, "querystring": true, "readline": true,
	"repl": true, "stream": true, "stream/consumers": true, "stream/promises": true,
	"stream/web": true, "string_decoder": true, "sys": true, "timers": true,
	"timers/promises": true, "tls": true, "trace_events": true, "tty": true,
	"url": true, "util": true, "util/types": true, "v8": true, "vm": true,
	"wasi": true, "worker_threads": true, "zlib": true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStringLiteral {
			s := strings.Trim(c.SourceText(), `"'`+"`")
			if nodeBuiltins[s] {
				ctx.Report(c, "import \""+s+"\" — prefer \"node:"+s+"\"")
			}
		}
		return false
	})
}

func visitRequire(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier || callee.SourceText() != "require" {
		return
	}
	// First arg.
	var firstArg *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && firstArg == nil && c.Kind() != wrapperchecker.KindTypeReference {
			firstArg = c
		}
		idx++
		return false
	})
	if firstArg == nil {
		return
	}
	if firstArg.Kind() != wrapperchecker.KindStringLiteral && firstArg.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return
	}
	s := strings.Trim(firstArg.SourceText(), `"'`+"`")
	if nodeBuiltins[s] {
		ctx.Report(firstArg, "require(\""+s+"\") — prefer \"node:"+s+"\"")
	}
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}
