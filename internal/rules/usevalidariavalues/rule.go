// Package usevalidariavalues implements use-valid-aria-values: ARIA
// attributes use a constrained value space (enums, IDREFs, numbers,
// booleans). A typo or invalid value just gets ignored by AT, which
// is worse than the property not being there.
package usevalidariavalues

import (
	"slices"
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-valid-aria-values"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxAttribute: visit,
	}
}

// Allowed enum values per ARIA attribute.
var enumValues = map[string][]string{
	"aria-autocomplete":   {"inline", "list", "both", "none"},
	"aria-checked":        {"true", "false", "mixed"},
	"aria-current":        {"true", "false", "page", "step", "location", "date", "time"},
	"aria-dropeffect":     {"copy", "move", "link", "execute", "popup", "none"},
	"aria-haspopup":       {"true", "false", "menu", "listbox", "tree", "grid", "dialog"},
	"aria-invalid":        {"true", "false", "grammar", "spelling"},
	"aria-live":           {"off", "polite", "assertive"},
	"aria-orientation":    {"horizontal", "vertical", "undefined"},
	"aria-pressed":        {"true", "false", "mixed"},
	"aria-relevant":       {"additions", "removals", "text", "all"},
	"aria-selected":       {"true", "false"},
	"aria-sort":           {"ascending", "descending", "none", "other"},
	"aria-expanded":       {"true", "false"},
	"aria-hidden":         {"true", "false"},
	"aria-modal":          {"true", "false"},
	"aria-multiline":      {"true", "false"},
	"aria-multiselectable": {"true", "false"},
	"aria-readonly":       {"true", "false"},
	"aria-required":       {"true", "false"},
	"aria-atomic":         {"true", "false"},
	"aria-busy":           {"true", "false"},
	"aria-disabled":       {"true", "false"},
	"aria-grabbed":        {"true", "false", "undefined"},
}

// Attributes that must be a non-empty IDREF list.
var idrefAttrs = map[string]bool{
	"aria-activedescendant": true,
	"aria-controls":         true,
	"aria-describedby":      true,
	"aria-details":          true,
	"aria-errormessage":     true,
	"aria-flowto":           true,
	"aria-labelledby":       true,
	"aria-owns":             true,
}

// Numeric attributes.
var numericAttrs = map[string]bool{
	"aria-level":       true,
	"aria-posinset":    true,
	"aria-setsize":     true,
	"aria-valuemax":    true,
	"aria-valuemin":    true,
	"aria-valuenow":    true,
	"aria-colcount":    true,
	"aria-colindex":    true,
	"aria-colspan":     true,
	"aria-rowcount":    true,
	"aria-rowindex":    true,
	"aria-rowspan":     true,
}

func visit(ctx *engine.Context, attr *wrapperchecker.Node) {
	name := jsxutil.AttributeName(attr)
	if !strings.HasPrefix(name, "aria-") {
		return
	}
	v, ok := jsxutil.AttributeStringValue(attr)
	if !ok {
		return
	}
	if idrefAttrs[name] {
		if strings.TrimSpace(v) == "" {
			ctx.Report(attr, name+" needs a non-empty id reference")
		}
		return
	}
	if numericAttrs[name] {
		if _, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil {
			ctx.Report(attr, name+"=\""+v+"\" isn't a number")
		}
		return
	}
	if allowed, ok := enumValues[name]; ok {
		// aria-relevant and aria-dropeffect accept space-separated tokens.
		tokens := strings.Fields(v)
		if len(tokens) == 0 {
			ctx.Report(attr, name+" needs a value")
			return
		}
		for _, t := range tokens {
			if !slices.Contains(allowed, t) {
				ctx.Report(attr, name+"=\""+v+"\" isn't a valid value")
				return
			}
		}
	}
}
