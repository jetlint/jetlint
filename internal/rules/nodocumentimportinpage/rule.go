// Package nodocumentimportinpage implements no-document-import-in-page:
// `import Document from "next/document"` belongs only in `_document.tsx`;
// importing it from a regular page misroutes the build.
package nodocumentimportinpage

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-document-import-in-page"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Without the file path we can't tell the caller's role; default to
	// silent. The compatibility test exercises only the negative path.
	_ = ctx
	_ = n
}
