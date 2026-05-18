// Package noimportcycles implements the no-import-cycles rule: a
// module must not be reachable from itself through a chain of
// non-type-only imports. Catches the classic A→B→A round-trip and
// longer A→B→C→A chains. Self-imports (A→A) are not flagged —
// biome's rule treats them as harmless, presumably because they are
// usually a re-export shim or a stale path that the resolver doesn't
// follow.
//
// The rule walks every ImportDeclaration and side-effecting
// ExportDeclaration (`export {} from`, `export * from`) in the
// current file and, for each one whose specifier resolves to another
// file in the program, asks: is the importing file reachable from
// the target through the program's import graph? If yes, the edge
// closes a cycle and the specifier is flagged.
//
// Per-file traversal does not memoise the program-wide graph because
// the engine instantiates a fresh Context per node; we rebuild edges
// once per visited SourceFile. For the directory-sized programs the
// rule sees in practice (single package, dozens to hundreds of
// files) this is acceptable; if it becomes a hotspot the natural
// move is to memoise on the *Program pointer.
package noimportcycles

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-import-cycles"

// Options configures the rule. IgnoreTypes mirrors biome's option of
// the same name: when true (default), `import type ...` declarations
// are stripped before cycle detection because TypeScript erases them
// at compile time and they do not exist at runtime.
type Options struct {
	IgnoreTypes bool
}

// DefaultOptions matches biome's defaults.
func DefaultOptions() Options { return Options{IgnoreTypes: true} }

// New constructs the rule with default options.
func New() engine.Rule { return &rule{opts: DefaultOptions()} }

// NewWithOptions constructs the rule with the supplied options.
func NewWithOptions(o Options) engine.Rule { return &rule{opts: o} }

// OptionsFromJSON parses the rule's configuration blob. Unknown keys
// are rejected so typos in `.jetlintrc.json` surface at startup
// rather than silently disabling a check.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 {
		return out, nil
	}
	type optsJSON struct {
		IgnoreTypes *bool `json:"ignoreTypes"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var parsed optsJSON
	if err := dec.Decode(&parsed); err != nil {
		return out, fmt.Errorf("no-import-cycles options: %w", err)
	}
	if parsed.IgnoreTypes != nil {
		out.IgnoreTypes = *parsed.IgnoreTypes
	}
	return out, nil
}

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, sf *wrapperchecker.Node) {
	prog := ctx.Program()
	if prog == nil {
		return
	}
	selfPath, _, _, _, _ := sf.SourceRange()
	if selfPath == "" {
		return
	}

	edges := buildEdges(prog, r.opts.IgnoreTypes)

	sf.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		spec := importSpecifier(stmt, r.opts.IgnoreTypes)
		if spec == nil {
			return true
		}
		target := resolveImport(prog, selfPath, spec.LiteralText())
		if target == "" || target == selfPath {
			return true
		}
		if reachable(edges, target, selfPath) {
			ctx.Report(spec, "This import is part of a cycle.")
		}
		return true
	})
}

// importSpecifier returns the module-specifier node of stmt iff stmt
// is an import or side-effecting export declaration that the rule
// should consider an edge in the import graph. Returns nil for any
// other statement, for declarations without a `from` clause, and
// (when ignoreTypes is true) for type-only declarations.
func importSpecifier(stmt *wrapperchecker.Node, ignoreTypes bool) *wrapperchecker.Node {
	switch stmt.Kind() {
	case wrapperchecker.KindImportDeclaration:
		if ignoreTypes && isTypeOnlyImport(stmt) {
			return nil
		}
		return stmt.ModuleSpecifier()
	case wrapperchecker.KindExportDeclaration:
		if ignoreTypes && stmt.IsTypeOnlyExport() {
			return nil
		}
		return stmt.ModuleSpecifier()
	}
	return nil
}

// isTypeOnlyImport reports whether an ImportDeclaration is `import
// type ...`. The wrapper exposes IsTypeOnlyExport but not the import
// equivalent yet, so we read the leading source text and check the
// keyword. Inline `import { type X }` is NOT type-only at the
// declaration level — biome treats it as a value import, matching
// TypeScript's emit rules (the inline `type` markers are erased per
// specifier but the import statement still runs at runtime).
func isTypeOnlyImport(n *wrapperchecker.Node) bool {
	text := n.SourceText()
	i := 0
	for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
		i++
	}
	const kw = "import"
	if !strings.HasPrefix(text[i:], kw) {
		return false
	}
	i += len(kw)
	if i >= len(text) || !isSpace(text[i]) {
		return false
	}
	for i < len(text) && isSpace(text[i]) {
		i++
	}
	const tkw = "type"
	if !strings.HasPrefix(text[i:], tkw) {
		return false
	}
	j := i + len(tkw)
	if j >= len(text) {
		return false
	}
	// `import type X`, `import type {`, `import type *`, `import type "..."`
	c := text[j]
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
		c == '{' || c == '*' || c == '"' || c == '\''
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// buildEdges constructs the program's import graph as an adjacency
// map keyed by absolute file path. Each edge points from importer to
// imported file, restricted to in-program targets — externals are
// ignored because they cannot close a user-space cycle.
func buildEdges(prog *wrapperchecker.Program, ignoreTypes bool) map[string][]string {
	out := map[string][]string{}
	for _, f := range prog.SourceFiles() {
		from := f.Path()
		var deps []string
		f.Walk(func(n *wrapperchecker.Node) bool {
			switch n.Kind() {
			case wrapperchecker.KindSourceFile:
				return true
			case wrapperchecker.KindImportDeclaration,
				wrapperchecker.KindExportDeclaration:
				spec := importSpecifier(n, ignoreTypes)
				if spec == nil {
					return false
				}
				target := resolveImport(prog, from, spec.LiteralText())
				if target == "" || target == from {
					return false
				}
				deps = append(deps, target)
				return false
			default:
				return false
			}
		})
		if len(deps) > 0 {
			out[from] = deps
		}
	}
	return out
}

// resolveImport maps a relative specifier from one source file to the
// absolute path of another file in the program. Bare specifiers
// (`react`, `@scope/pkg`) are returned as "" because they cannot
// resolve to a user file. Bundler-style extension fixups are
// honoured: `./x.js` resolves to `./x.ts` when only the TypeScript
// file is present, mirroring `moduleResolution: bundler`.
func resolveImport(prog *wrapperchecker.Program, fromPath, spec string) string {
	if spec == "" {
		return ""
	}
	if !strings.HasPrefix(spec, ".") && !strings.HasPrefix(spec, "/") {
		return ""
	}
	fromDir := filepath.Dir(fromPath)
	abs := filepath.Clean(filepath.Join(fromDir, spec))
	if prog.SourceFileByPath(abs) != nil {
		return abs
	}
	ext := filepath.Ext(abs)
	stem := strings.TrimSuffix(abs, ext)
	candidates := []string{}
	switch ext {
	case ".js", ".mjs", ".cjs", ".jsx":
		candidates = append(candidates,
			stem+".ts", stem+".tsx", stem+".d.ts", stem+".mts", stem+".cts")
	case ".ts", ".mts", ".cts", ".tsx", ".d.ts":
		// Exact path already attempted; nothing else to try.
	default:
		candidates = append(candidates,
			abs+".ts", abs+".tsx", abs+".d.ts",
			abs+".js", abs+".jsx", abs+".mts", abs+".cts",
			filepath.Join(abs, "index.ts"), filepath.Join(abs, "index.tsx"),
			filepath.Join(abs, "index.js"), filepath.Join(abs, "index.jsx"))
	}
	for _, c := range candidates {
		if prog.SourceFileByPath(c) != nil {
			return c
		}
	}
	return ""
}

// reachable reports whether there exists a non-empty path of import
// edges from start to target. start itself is not considered the
// target — only nodes discovered while traversing outward count, so
// the search correctly answers "does the import graph close a cycle
// back to target?" without false-positive self-matches at the seed.
func reachable(edges map[string][]string, start, target string) bool {
	if start == target {
		// Direct self-edge from the seed is a single-step round trip,
		// but the caller already rejected self-imports at the source.
		// Treat this as reachable for any indirect call.
		return true
	}
	seen := map[string]bool{start: true}
	stack := append([]string(nil), edges[start]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == target {
			return true
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		stack = append(stack, edges[n]...)
	}
	return false
}
