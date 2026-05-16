// Package noprivateimports implements no-private-imports: JSDoc
// `@private`, `@package`, and `@public` tags (or `@access X`) on an
// `export` declare the visibility scope of that symbol. Importing a
// `@private` symbol from a different file or a `@package` symbol
// from outside its package directory is flagged. The default
// visibility for un-annotated exports is configurable
// (defaultVisibility option, `"public"` or `"package"`).
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
	exportVis := loadExportVisibility(res.File, defaultVis)
	if exportVis == nil {
		return
	}
	importerFile := importerFilePath(imp)
	for _, b := range importedBindings(imp) {
		vis, ok := exportVis[b.imported]
		if !ok {
			continue
		}
		if !canSee(vis, importerFile, res.File) {
			ctx.Report(b.node, visibilityMessage(vis))
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

func canSee(v visibility, importer, target string) bool {
	switch v {
	case visPublic:
		return true
	case visPrivate:
		return importer == target
	case visPackage:
		return inSameOrSubPackage(importer, target)
	}
	return true
}

// inSameOrSubPackage returns true when importer is in the same
// directory as target or in any subdirectory. Package boundaries are
// directories; an importer inside the package's tree may freely
// import @package symbols defined at the package root.
func inSameOrSubPackage(importer, target string) bool {
	targetDir := filepath.Dir(target)
	importerDir := filepath.Dir(importer)
	rel, err := filepath.Rel(targetDir, importerDir)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
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
	// `a as b` → [a, b]; imported = a. `a` alone → imported = a.
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

// loadExportVisibility reads the target file from disk and returns
// a name→visibility map for every named export. Reads the file once
// per call; the program is small enough that we don't bother caching
// across rule invocations.
func loadExportVisibility(path string, defaultVis visibility) map[string]visibility {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(data)
	out := map[string]visibility{}
	for _, e := range scanExports(src) {
		vis := jsdocVisibility(src[:e.start], defaultVis)
		out[e.name] = vis
	}
	return out
}

type exportInfo struct {
	name  string
	start int
}

// exportPattern matches the various forms of named exports we care
// about. Captured group 1 is the exported name. Spans we don't try
// to handle: `export *`, re-exports (`export { x } from`), default
// exports (their visibility is keyed off "default" by the import
// side anyway).
var exportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+class\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+interface\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+type\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+enum\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
}

var defaultExportPattern = regexp.MustCompile(`(?m)^\s*export\s+default\s+(?:async\s+)?function`)

func scanExports(src string) []exportInfo {
	seen := map[int]bool{}
	var out []exportInfo
	for _, re := range exportPatterns {
		for _, m := range re.FindAllStringSubmatchIndex(src, -1) {
			start := m[0]
			name := src[m[2]:m[3]]
			if seen[start] {
				continue
			}
			seen[start] = true
			out = append(out, exportInfo{name: name, start: start})
		}
	}
	for _, m := range defaultExportPattern.FindAllStringIndex(src, -1) {
		if seen[m[0]] {
			continue
		}
		seen[m[0]] = true
		out = append(out, exportInfo{name: "default", start: m[0]})
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
