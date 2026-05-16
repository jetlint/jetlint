// Package useimportextensions implements use-import-extensions: a
// relative import should spell the on-disk file extension verbatim
// (`./foo.ts`, not `./foo`). Bundler-style resolution silently
// guesses extensions and that guess can drift when files move, so
// biome — and Node's own ESM loader — want the extension explicit.
// The rule fires when the specifier resolves to a real file but
// the literal path on the import statement doesn't already point
// at that file as-typed.
package useimportextensions

import (
	"os"
	"path/filepath"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/modresolve"
)

const id = "use-import-extensions"

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
	checkSpecifier(ctx, spec)
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
	for arg.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		arg.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner == nil {
			return
		}
		arg = inner
	}
	if arg.Kind() != wrapperchecker.KindStringLiteral {
		return
	}
	checkSpecifier(ctx, arg)
}

func checkSpecifier(ctx *engine.Context, spec *wrapperchecker.Node) {
	if spec == nil || spec.Kind() != wrapperchecker.KindStringLiteral {
		return
	}
	text := spec.LiteralText()
	if !modresolve.IsRelativeSpecifier(text) {
		return
	}
	// Strip query/hash suffixes; bundlers carry them through but
	// they aren't part of the on-disk file name.
	pathPart := text
	if i := strings.IndexAny(pathPart, "?#"); i >= 0 {
		pathPart = pathPart[:i]
	}
	sourceFile, _, _, _, _ := spec.SourceRange()
	if sourceFile == "" {
		return
	}
	abs := filepath.Join(filepath.Dir(sourceFile), pathPart)
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		// The literal path resolves to a file as-typed — nothing to add.
		return
	}
	// Fall back to extension-guessing: if any common extension or
	// `/index.<ext>` form points at a real file, the import would
	// resolve with that ext and the user should write it explicitly.
	// This sidesteps the checker's "is this a module" filter, which
	// excludes script-style files with no exports.
	if !resolvesByGuessingExtensions(abs) {
		return
	}
	ctx.Report(spec, "add the on-disk file extension to this relative import")
}

// candidateExtensions covers the file types tsc/bundlers will try
// when extension-less imports are encountered. The list matches
// biome's order so we surface the same "winner" as the upstream
// rule when multiple candidates exist.
var candidateExtensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"}

func resolvesByGuessingExtensions(abs string) bool {
	for _, ext := range candidateExtensions {
		if info, err := os.Stat(abs + ext); err == nil && !info.IsDir() {
			return true
		}
	}
	// Directory imports resolve to <dir>/index.<ext>.
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		for _, ext := range candidateExtensions {
			candidate := filepath.Join(abs, "index"+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return true
			}
		}
	}
	return false
}
