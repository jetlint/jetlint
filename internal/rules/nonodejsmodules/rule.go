// Package nonodejsmodules implements no-nodejs-modules: flag any
// import of a Node.js core module (`fs`, `path`, `crypto`, …,
// optionally prefixed with `node:`). The rule is for code that
// targets browsers, Deno, or another non-Node runtime where these
// imports would fail at load time.
//
// Type-only imports are intentionally allowed — `import type fs from "fs"`
// pulls in types without bundling the module. `declare module "..."`
// blocks are also skipped because they augment ambient module
// declarations rather than pulling code in.
//
// `require("...")` calls and dynamic `import("...")` expressions
// are both treated as imports.
package nonodejsmodules

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-nodejs-modules"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visitImport,
		wrapperchecker.KindCallExpression:    visitCall,
	}
}

func visitImport(ctx *engine.Context, n *wrapperchecker.Node) {
	if isTypeOnlyImport(n) {
		return
	}
	spec := n.ModuleSpecifier()
	if spec == nil || spec.Kind() != wrapperchecker.KindStringLiteral {
		return
	}
	report(ctx, n, spec.LiteralText())
}

// visitCall handles `require("fs")` (CommonJS) and `import("fs")`
// (dynamic ESM). Both surface as CallExpressions in the AST; the
// callee differs (an Identifier `require` vs. the ImportKeyword
// `import`).
func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	switch {
	case callee.Kind() == wrapperchecker.KindIdentifier && callee.LiteralText() == "require":
		// ok
	case callee.Kind() == wrapperchecker.KindImportKeyword:
		// ok
	default:
		return
	}
	args := n.CallArguments()
	if len(args) == 0 {
		return
	}
	a := args[0]
	if a.Kind() != wrapperchecker.KindStringLiteral &&
		a.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return
	}
	report(ctx, n, a.LiteralText())
}

func report(ctx *engine.Context, n *wrapperchecker.Node, spec string) {
	if !isNodeCoreModule(spec) {
		return
	}
	ctx.Report(n, "Node.js core module '"+spec+"' is not portable to non-Node runtimes")
}

// isTypeOnlyImport reports whether an ImportDeclaration is
// `import type ...`. The wrapper exposes this via a flag on the
// ImportClause child; we look it up via SourceText for portability.
func isTypeOnlyImport(n *wrapperchecker.Node) bool {
	src := n.SourceText()
	src = strings.TrimLeft(src, " \t\n\r")
	return strings.HasPrefix(src, "import type") || strings.HasPrefix(src, "import\ttype")
}

// isNodeCoreModule reports whether the import specifier names a
// Node.js builtin. Accepts both the bare name (`fs`) and the
// `node:` protocol (`node:fs`), and sub-paths (`fs/promises`).
func isNodeCoreModule(spec string) bool {
	if rest, ok := strings.CutPrefix(spec, "node:"); ok {
		spec = rest
	}
	// Split off any sub-path; the lookup table holds the root
	// module names.
	if i := strings.IndexByte(spec, '/'); i >= 0 {
		spec = spec[:i]
	}
	return nodeBuiltins[spec]
}

var nodeBuiltins = map[string]bool{
	"assert":              true,
	"async_hooks":         true,
	"buffer":              true,
	"child_process":       true,
	"cluster":             true,
	"console":             true,
	"constants":           true,
	"crypto":              true,
	"dgram":               true,
	"diagnostics_channel": true,
	"dns":                 true,
	"domain":              true,
	"events":              true,
	"fs":                  true,
	"http":                true,
	"http2":               true,
	"https":               true,
	"inspector":           true,
	"module":              true,
	"net":                 true,
	"os":                  true,
	"path":                true,
	"perf_hooks":          true,
	"process":             true,
	"punycode":            true,
	"querystring":         true,
	"readline":            true,
	"repl":                true,
	"stream":              true,
	"string_decoder":      true,
	"sys":                 true,
	"timers":              true,
	"tls":                 true,
	"trace_events":        true,
	"tty":                 true,
	"url":                 true,
	"util":                true,
	"v8":                  true,
	"vm":                  true,
	"wasi":                true,
	"worker_threads":      true,
	"zlib":                true,
}
