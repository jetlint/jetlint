// Package relatedgettersetterpairs implements the related-getter-setter-pairs rule.
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint. This is a scaffolded stub — the rule does
// not currently emit diagnostics. Implement by adding handlers for the
// relevant AST kinds and reading the upstream test fixtures as a
// black-box spec.
package relatedgettersetterpairs

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "related-getter-setter-pairs"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{}
}
