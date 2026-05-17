// Package noprivateimports implements no-private-imports: JSDoc
// `@private`, `@package`, and `@public` tags (or `@access X`) on an
// `export` declare the visibility scope of that symbol. Importing a
// `@private` symbol from a different file or a `@package` symbol
// from outside its package directory is flagged. Re-exports follow
// the original definition's visibility, with the wrinkle that a
// `@package` symbol gains a new permitted-import directory each time
// it's re-exported — so `import { x } from "./sub"` (where `./sub`
// is `./sub/index.ts` re-exporting from `./sub/foo.ts`) widens the
// allowed boundary from `./sub/` to also include the directory
// containing `./sub/index.ts`. The default visibility for
// un-annotated exports is configurable (defaultVisibility option,
// `"public"` or `"package"`).
package noprivateimports

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/modresolve"
)

const id = "no-private-imports"

// Options carries the rule's user-tunable knobs.
type Options struct {
	// DefaultVisibility is applied to un-annotated exports. Allowed
	// values are "public" (the default) and "package".
	DefaultVisibility string
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visitImport,
	}
}

type visibility int

const (
	visPublic visibility = iota
	visPackage
	visPrivate
)

// visInfo is the resolved visibility for a symbol as seen from a
// particular import. For @package symbols, allowedDirs holds every
// directory subtree from which the symbol may be imported — the
// original definition's directory, plus one entry per re-export
// site reached on the way to it.
type visInfo struct {
	vis         visibility
	defFile     string
	allowedDirs []string
}

func visitImport(ctx *engine.Context, imp *wrapperchecker.Node) {
	specifier := imp.ModuleSpecifier()
	if specifier == nil {
		return
	}
	res := modresolve.Resolve(ctx.Checker(), specifier)
	if !res.Resolved || res.File == "" {
		return
	}
	defaultVis := defaultVisibility(ctx)
	importerFile := importerFilePath(imp)
	for _, b := range importedBindings(imp) {
		info := resolveVisibility(res.File, b.imported, defaultVis, map[string]bool{})
		if info == nil {
			continue
		}
		if !canSee(info, importerFile) {
			ctx.Report(b.node, visibilityMessage(info.vis))
		}
	}
}

func defaultVisibility(ctx *engine.Context) visibility {
	opts, _ := ctx.Options().(*Options)
	if opts == nil {
		return visPublic
	}
	switch strings.ToLower(opts.DefaultVisibility) {
	case "package":
		return visPackage
	case "private":
		return visPrivate
	}
	return visPublic
}

func visibilityMessage(v visibility) string {
	switch v {
	case visPrivate:
		return "You may not import a symbol with private visibility from here."
	case visPackage:
		return "You may not import a symbol with package visibility from here."
	}
	return "You may not import this symbol from here."
}

func canSee(info *visInfo, importer string) bool {
	if info.vis == visPublic {
		return true
	}
	if importer == info.defFile {
		return true
	}
	importerDir := filepath.Dir(importer)
	for _, dir := range info.allowedDirs {
		rel, err := filepath.Rel(dir, importerDir)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

// resolveVisibility looks up the visibility of `name` as exported by
// `file`, following re-export chains. Returns nil when neither a
// direct definition nor a re-export of `name` exists in `file` (the
// rule simply has no opinion in that case). The `seen` set guards
// against import cycles.
func resolveVisibility(file, name string, defaultVis visibility, seen map[string]bool) *visInfo {
	if file == "" || seen[file] {
		return nil
	}
	seen[file] = true
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	src := string(data)
	if info := directDefinitionVisibility(file, src, name, defaultVis); info != nil {
		return info
	}
	for _, r := range reExportsOf(src, name) {
		targetFile := resolveRelative(file, r.fromModule)
		if targetFile == "" {
			continue
		}
		// Pass a shallow copy of seen so sibling re-exports don't
		// interfere with each other's traversal.
		nestedSeen := make(map[string]bool, len(seen))
		for k, v := range seen {
			nestedSeen[k] = v
		}
		info := resolveVisibility(targetFile, r.originalName, defaultVis, nestedSeen)
		if info == nil {
			continue
		}
		// Re-exporting widens the import boundary, but the size of
		// the widening depends on the symbol's visibility:
		//
		//   @package: the re-export brings the symbol into the
		//   package containing the re-export file, so importers
		//   anywhere in that *parent* directory can resolve it
		//   via `from "./<pkg>"`.
		//
		//   @private: looser only insofar as the importer is in
		//   the same folder as the index file (or descended into
		//   it) — biome treats `import ... from "./index"` from
		//   within the package as an allowed shortcut for the
		//   private symbol.
		switch info.vis {
		case visPackage:
			info.allowedDirs = append(info.allowedDirs, filepath.Dir(filepath.Dir(file)))
		case visPrivate:
			info.allowedDirs = append(info.allowedDirs, filepath.Dir(file))
		}
		return info
	}
	return nil
}

// directDefinitionVisibility returns a visInfo when `name` is
// directly defined and exported in `src`, otherwise nil.
func directDefinitionVisibility(file, src, name string, defaultVis visibility) *visInfo {
	for _, e := range scanExports(src) {
		if e.name != name {
			continue
		}
		vis := jsdocVisibility(src[:e.start], defaultVis)
		info := &visInfo{vis: vis, defFile: file}
		if vis == visPackage {
			info.allowedDirs = []string{filepath.Dir(file)}
		}
		return info
	}
	return nil
}

// resolveRelative resolves a relative module specifier (`./foo`,
// `../bar`) from `fromFile` to an on-disk file path. Tries common
// TypeScript / JavaScript extensions and `<dir>/index.<ext>` fall-
// backs so `from "./sub"` resolves to `./sub/index.ts`.
func resolveRelative(fromFile, spec string) string {
	if !strings.HasPrefix(spec, ".") {
		return ""
	}
	base := filepath.Dir(fromFile)
	candidate := filepath.Join(base, spec)
	// Strip a trailing .js / .ts so we can try our own extension
	// list — biome's fixtures use `.js` even from `.ts` files.
	for _, ext := range []string{".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs"} {
		if strings.HasSuffix(candidate, ext) {
			stem := strings.TrimSuffix(candidate, ext)
			if p := firstExistingFile(stem); p != "" {
				return p
			}
		}
	}
	if p := firstExistingFile(candidate); p != "" {
		return p
	}
	// Try as a directory with an index file.
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".d.ts", ".mjs", ".cjs"} {
		p := filepath.Join(candidate, "index"+ext)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func firstExistingFile(stem string) string {
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".d.ts", ".mjs", ".cjs"} {
		p := stem + ext
		if fileExists(p) {
			return p
		}
	}
	if fileExists(stem) {
		return stem
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

type importedBinding struct {
	node     *wrapperchecker.Node
	imported string
}

func importedBindings(imp *wrapperchecker.Node) []importedBinding {
	var clause *wrapperchecker.Node
	imp.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindImportClause {
			clause = c
			return true
		}
		return false
	})
	if clause == nil {
		return nil
	}
	var out []importedBinding
	clause.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			// default import: `import Foo from "x"`
			out = append(out, importedBinding{node: c, imported: "default"})
		case wrapperchecker.KindNamedImports:
			c.ForEachChild(func(spec *wrapperchecker.Node) bool {
				if spec.Kind() == wrapperchecker.KindImportSpecifier {
					if name, node := importSpecifierImportedName(spec); name != "" {
						out = append(out, importedBinding{node: node, imported: name})
					}
				}
				return false
			})
		}
		return false
	})
	return out
}

// importSpecifierImportedName returns the *original* exported name
// the specifier refers to and the AST node that names it for
// diagnostic anchoring. `{ a as b }` → ("a", node-for-a).
func importSpecifierImportedName(spec *wrapperchecker.Node) (string, *wrapperchecker.Node) {
	var names []*wrapperchecker.Node
	spec.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			names = append(names, c)
		}
		return false
	})
	if len(names) == 0 {
		return "", nil
	}
	first := names[0]
	return first.LiteralText(), first
}

func importerFilePath(n *wrapperchecker.Node) string {
	root := n
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return ""
	}
	file, _, _, _, _ := root.SourceRange()
	return file
}

type exportInfo struct {
	name  string
	start int
}

// exportPattern matches the various forms of named direct-definition
// exports. Captured group 1 is the exported name. Re-exports are
// handled separately by reExportsOf below.
var exportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+class\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+interface\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+type\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+enum\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
}

var defaultExportPattern = regexp.MustCompile(`(?m)^\s*export\s+default\s+(?:async\s+)?`)

func scanExports(src string) []exportInfo {
	var out []exportInfo
	addedAt := map[int]map[string]bool{}
	add := func(name string, start int) {
		if addedAt[start] == nil {
			addedAt[start] = map[string]bool{}
		}
		if addedAt[start][name] {
			return
		}
		addedAt[start][name] = true
		out = append(out, exportInfo{name: name, start: start})
	}
	for _, re := range exportPatterns {
		for _, m := range re.FindAllStringSubmatchIndex(src, -1) {
			add(src[m[2]:m[3]], m[0])
		}
	}
	for _, m := range defaultExportPattern.FindAllStringIndex(src, -1) {
		add("default", m[0])
	}
	return out
}

// reExport is one entry in an `export { ... } from "module"` clause.
type reExport struct {
	originalName string // the name as exported by `fromModule`
	fromModule   string // the source module specifier (without quotes)
}

// reExportPattern matches `export { ... } from "module"`. Captured
// groups: 1 = inner specifier list, 2 = module specifier text.
var reExportPattern = regexp.MustCompile(`(?s)export\s*\{([^}]*)\}\s*from\s*["']([^"']+)["']`)

// reExportSpecifierPattern matches one specifier inside a re-export's
// `{ ... }` clause: `name` or `name as alias`. Captured groups:
// 1 = original name, 2 = local/alias name (or empty when there's no `as`).
var reExportSpecifierPattern = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)(?:\s+as\s+([A-Za-z_$][A-Za-z0-9_$]*))?`)

// reExportsOf returns every re-export of `name` from `src`. A
// re-export contributes when its locally-exported name (the name
// after `as`, or the raw name if there's no `as`) equals `name`.
// The returned originalName is the name as known to the source
// module being re-exported from.
func reExportsOf(src, name string) []reExport {
	var out []reExport
	for _, m := range reExportPattern.FindAllStringSubmatch(src, -1) {
		inner := m[1]
		module := m[2]
		for _, sm := range reExportSpecifierPattern.FindAllStringSubmatch(inner, -1) {
			orig := sm[1]
			local := sm[2]
			if local == "" {
				local = orig
			}
			if local == name {
				out = append(out, reExport{originalName: orig, fromModule: module})
			}
		}
	}
	return out
}

// jsdocVisibility scans the textual region immediately preceding an
// export for the closest `/** ... */` block and pulls a visibility
// out of its tags. Returns defaultVis if no tag is found.
var (
	jsdocBlockPattern = regexp.MustCompile(`(?s)/\*\*.*?\*/`)
	jsdocPrivateTag   = regexp.MustCompile(`@private\b|@access\s+private\b`)
	jsdocPackageTag   = regexp.MustCompile(`@package\b|@access\s+package\b`)
	jsdocPublicTag    = regexp.MustCompile(`@public\b|@access\s+public\b`)
)

func jsdocVisibility(prefix string, defaultVis visibility) visibility {
	matches := jsdocBlockPattern.FindAllString(prefix, -1)
	if len(matches) == 0 {
		return defaultVis
	}
	block := matches[len(matches)-1]
	switch {
	case jsdocPrivateTag.MatchString(block):
		return visPrivate
	case jsdocPackageTag.MatchString(block):
		return visPackage
	case jsdocPublicTag.MatchString(block):
		return visPublic
	}
	return defaultVis
}
