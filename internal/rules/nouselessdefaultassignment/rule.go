// Package nouselessdefaultassignment implements the no-useless-default-assignment
// rule: flag default-assignments where the default literally cannot be
// reached because the destructured/assigned target's type excludes
// undefined.
package nouselessdefaultassignment

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-useless-default-assignment"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{}
}
