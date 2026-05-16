// Package eslintcompat loads JSON fixture files produced by
// cmd/oxlint-fixtures from oxc's ESLint-core rule sources. Each fixture
// is a flat list of {code, valid} cases; per-rule harnesses run the
// case code through jetlint and compare the diagnostic count against
// the valid/invalid flag.
//
// This is the ESLint-core counterpart to internal/tselintcompat, which
// loads typescript-eslint's RuleTester TypeScript fixtures. ESLint
// core does not publish a machine-readable fixture format, so we
// vendor oxlint's instead — they exercise the same upstream behavior.
package eslintcompat

import (
	"encoding/json"
	"fmt"
	"os"
)

// Case is one extracted oxlint fixture case.
type Case struct {
	// Code is the source text the rule is being run against.
	// For multi-file cases, leave Code empty and use Files+Main.
	Code string `json:"code"`
	// Valid is true for entries from oxc's `pass` vec, false for `fail`.
	Valid bool `json:"valid"`
	// Options is the ESLint-style options array for the case
	// (`[options0, options1, ...]`). Nil when the upstream case had no
	// options or used `None`. Per-rule harnesses typically read
	// FirstOption() and convert it to a typed Options struct for
	// NewWithOptions(...).
	Options []any `json:"options,omitempty"`
	// Settings is the ESLint-style settings object captured from
	// oxc's third tuple position. Typically supplies `globals` (a
	// `{ name: bool }` map naming globals the rule should treat as
	// declared) or `env`. Nil when the upstream case had no settings.
	Settings map[string]any `json:"settings,omitempty"`
	// Files lists every file the case needs on disk. For rules that
	// reach across files (import resolution, package.json discovery)
	// this carries the sibling fixtures alongside the entry point.
	// Code-only cases leave it nil.
	Files []File `json:"files,omitempty"`
	// Main names the entry file the linter should treat as the case
	// under test when Files is set. Defaults to the first file when
	// blank.
	Main string `json:"main,omitempty"`
}

// File is one file in a multi-file case.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Globals returns the `globals` map from Settings, or nil when none
// is present. Values are kept as their original Rust JSON form (bool
// for "readonly"/"writable" in ESLint terms).
func (c Case) Globals() map[string]any {
	if c.Settings == nil {
		return nil
	}
	g, _ := c.Settings["globals"].(map[string]any)
	return g
}

// FirstOption returns Options[0] as a map[string]any (the shape rules
// almost always consume), or nil when no options are present or
// Options[0] is not an object.
func (c Case) FirstOption() map[string]any {
	if len(c.Options) == 0 {
		return nil
	}
	obj, _ := c.Options[0].(map[string]any)
	return obj
}

// Fixture is the on-disk shape produced by cmd/oxlint-fixtures.
type Fixture struct {
	RuleID    string `json:"ruleId"`
	OxcSHA    string `json:"oxcSHA,omitempty"`
	OxcSource string `json:"oxcSource"`
	Cases     []Case `json:"cases"`
}

// Load reads and decodes the fixture file at path.
func Load(path string) (*Fixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	var fx Fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		return nil, fmt.Errorf("decode fixture: %w", err)
	}
	return &fx, nil
}
