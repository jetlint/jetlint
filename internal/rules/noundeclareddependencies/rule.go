// Package noundeclareddependencies implements
// no-undeclared-dependencies: a non-relative `import "foo"` that
// isn't declared in the nearest package.json will fail at install
// time on a clean checkout. The rule walks up the file tree to
// find the closest package.json, builds a set of declared package
// names from dependencies/peerDependencies/optionalDependencies
// (and optionally devDependencies), and flags any package import
// whose name isn't in that set — skipping Node/Bun builtins, the
// `node:` and `bun:` schemes, the current package's own name, and
// the usual path-alias prefixes.
package noundeclareddependencies

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/modresolve"
)

const id = "no-undeclared-dependencies"

// Options controls whether `devDependencies` are accepted. When
// false, an import of a package listed only under devDependencies
// will be flagged — typical for production source files that
// shouldn't pull in test/build-only packages at runtime.
type Options struct {
	DevDependencies bool
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visit,
		wrapperchecker.KindExportDeclaration: visit,
		wrapperchecker.KindCallExpression:    visitCall,
	}
}

func visit(ctx *engine.Context, decl *wrapperchecker.Node) {
	spec := decl.ModuleSpecifier()
	checkSpec(ctx, spec, isTypeOnlyImport(decl))
}

func visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil {
		return
	}
	switch callee.Kind() {
	case wrapperchecker.KindImportKeyword:
	case wrapperchecker.KindIdentifier:
		if callee.LiteralText() != "require" {
			return
		}
	default:
		return
	}
	var arg *wrapperchecker.Node
	seenCallee := false
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenCallee {
			seenCallee = true
			return false
		}
		if arg == nil {
			arg = c
			return true
		}
		return false
	})
	if arg == nil {
		return
	}
	checkSpec(ctx, arg, false)
}

func checkSpec(ctx *engine.Context, spec *wrapperchecker.Node, typeOnly bool) {
	if spec == nil || spec.Kind() != wrapperchecker.KindStringLiteral {
		return
	}
	text := spec.LiteralText()
	if modresolve.IsRelativeSpecifier(text) {
		return
	}
	if isPathAlias(text) || isSchemeImport(text) || isBuiltin(text) {
		return
	}
	pkg := packageName(text)
	if pkg == "" {
		return
	}
	sourceFile, _, _, _, _ := spec.SourceRange()
	if sourceFile == "" {
		return
	}
	manifest := findPackageJSON(sourceFile)
	if manifest == nil {
		return
	}
	if pkg == manifest.Name {
		return
	}
	if manifest.Has(pkg, true) {
		return
	}
	opts, _ := ctx.Options().(*Options)
	allowDev := opts == nil || opts.DevDependencies
	if allowDev && manifest.HasDev(pkg) {
		return
	}
	if typeOnly && manifest.HasDev("@types/"+pkg) {
		return
	}
	ctx.Report(spec, "this dependency is not declared in package.json")
}

// isPathAlias reports whether the specifier uses a common path-alias
// convention that won't resolve to a node_modules entry (`@/foo`,
// `#foo` for Node's package imports map, `~/foo`).
func isPathAlias(spec string) bool {
	if strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "#") || strings.HasPrefix(spec, "~/") {
		return true
	}
	return false
}

func isSchemeImport(spec string) bool {
	return strings.HasPrefix(spec, "node:") || strings.HasPrefix(spec, "bun:")
}

// nodeBuiltins enumerates Node.js builtin modules accepted without
// a `node:` prefix. The list is intentionally narrow — only the
// commonly-bare-imported ones — because anything else should
// already be written as `node:...`.
var nodeBuiltins = map[string]bool{
	"assert": true, "buffer": true, "child_process": true, "cluster": true,
	"console": true, "constants": true, "crypto": true, "dgram": true,
	"dns": true, "domain": true, "events": true, "fs": true, "http": true,
	"http2": true, "https": true, "module": true, "net": true, "os": true,
	"path": true, "perf_hooks": true, "process": true, "punycode": true,
	"querystring": true, "readline": true, "repl": true, "stream": true,
	"string_decoder": true, "sys": true, "timers": true, "tls": true,
	"tty": true, "url": true, "util": true, "v8": true, "vm": true,
	"wasi": true, "worker_threads": true, "zlib": true, "bun": true,
}

func isBuiltin(spec string) bool {
	return nodeBuiltins[spec]
}

// packageName returns the npm package name part of an import
// specifier, dropping any subpath segment. For scoped packages the
// scope is preserved (`@mui/material/Button` -> `@mui/material`).
func packageName(spec string) string {
	if spec == "" {
		return ""
	}
	if spec[0] == '@' {
		// Scoped: @scope/name[/subpath]
		parts := strings.SplitN(spec, "/", 3)
		if len(parts) < 2 {
			return ""
		}
		return parts[0] + "/" + parts[1]
	}
	if i := strings.Index(spec, "/"); i >= 0 {
		return spec[:i]
	}
	return spec
}

// isTypeOnlyImport reports whether the import statement is a
// type-only import (`import type ...` or `import type { ... }`).
// Type-only imports don't need a runtime dependency declaration,
// just a `@types/...` declaration in devDependencies.
func isTypeOnlyImport(decl *wrapperchecker.Node) bool {
	if decl.Kind() != wrapperchecker.KindImportDeclaration {
		return false
	}
	var typeOnly bool
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindImportClause {
			return false
		}
		text := c.SourceText()
		if strings.HasPrefix(text, "type ") || strings.HasPrefix(text, "type{") {
			typeOnly = true
			return true
		}
		return true
	})
	return typeOnly
}

// manifest is the slice of a package.json the rule cares about.
type manifest struct {
	Name             string
	Dependencies     map[string]string
	PeerDependencies map[string]string
	OptDependencies  map[string]string
	DevDependencies  map[string]string
}

// Has reports whether pkg appears in the non-dev dependency lists.
// When includeOptional/Peer is true, peer and optional are
// considered.
func (m *manifest) Has(pkg string, includeOptionalPeer bool) bool {
	if _, ok := m.Dependencies[pkg]; ok {
		return true
	}
	if includeOptionalPeer {
		if _, ok := m.PeerDependencies[pkg]; ok {
			return true
		}
		if _, ok := m.OptDependencies[pkg]; ok {
			return true
		}
	}
	return false
}

func (m *manifest) HasDev(pkg string) bool {
	_, ok := m.DevDependencies[pkg]
	return ok
}

// findPackageJSON walks up from the given source file looking for
// the closest package.json. Returns nil when no manifest is found.
func findPackageJSON(sourceFile string) *manifest {
	dir := filepath.Dir(sourceFile)
	for {
		path := filepath.Join(dir, "package.json")
		if data, err := os.ReadFile(path); err == nil {
			var raw struct {
				Name             string            `json:"name"`
				Dependencies     map[string]string `json:"dependencies"`
				PeerDependencies map[string]string `json:"peerDependencies"`
				OptDependencies  map[string]string `json:"optionalDependencies"`
				DevDependencies  map[string]string `json:"devDependencies"`
			}
			if err := json.Unmarshal(data, &raw); err == nil {
				return &manifest{
					Name:             raw.Name,
					Dependencies:     raw.Dependencies,
					PeerDependencies: raw.PeerDependencies,
					OptDependencies:  raw.OptDependencies,
					DevDependencies:  raw.DevDependencies,
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}
