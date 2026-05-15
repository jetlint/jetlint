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
	Code string `json:"code"`
	// Valid is true for entries from oxc's `pass` vec, false for `fail`.
	Valid bool `json:"valid"`
	// HasOptions is true when the upstream oxc case was paired with a
	// non-None options blob. Per-rule harnesses skip these so the
	// reported pass rate reflects default-option behavior only.
	HasOptions bool `json:"hasOptions,omitempty"`
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
