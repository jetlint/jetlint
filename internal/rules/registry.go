// Package rules is the registry of rules the linter ships. Concrete rule
// implementations register themselves via Register; for now the registry
// is seeded with the MVP rule names so configuration validation can
// distinguish a typo from a future rule. Rule logic lives in subpackages
// added as each rule lands.
package rules

import (
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// MVPRuleIDs is the canonical, ordered list of rule identifiers shipped
// in v0.1. The slice is the source of truth; helper functions derive
// maps from it as needed.
var MVPRuleIDs = []string{
	"no-floating-promises",
	"no-misused-promises",
	"strict-boolean-expressions",
	"no-unsafe-assignment",
	"no-base-to-string",
}

// DefaultSeverity returns the severity at which a known rule fires when
// no configuration overrides it. All MVP rules default to error so the
// out-of-the-box experience is "violations break the build".
func DefaultSeverity(ruleID string) wrapperlint.Severity {
	if !IsKnown(ruleID) {
		return ""
	}
	return wrapperlint.SeverityError
}

// IsKnown reports whether ruleID names a rule the linter ships.
func IsKnown(ruleID string) bool {
	for _, id := range MVPRuleIDs {
		if id == ruleID {
			return true
		}
	}
	return false
}
