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
// at v0.1 with full default-on behavior. These default to error
// severity so the out-of-the-box experience is "violations break the
// build".
var MVPRuleIDs = []string{
	"no-floating-promises",
	"no-misused-promises",
	"strict-boolean-expressions",
	"no-unsafe-assignment",
	"no-base-to-string",
}

// AdditionalTypeAwareRuleIDs is every type-aware rule beyond the MVP
// set that ships scaffolded (and in many cases fully implemented).
// These default to "off" severity — opt-in via .tsgolintrc.json — so
// adding new rules to the binary doesn't silently change project
// behavior. As each rule reaches production-ready completeness it can
// be promoted to MVPRuleIDs.
var AdditionalTypeAwareRuleIDs = []string{
	"await-thenable",
	"consistent-return",
	"consistent-type-exports",
	"dot-notation",
	"naming-convention",
	"no-array-delete",
	"no-confusing-void-expression",
	"no-deprecated",
	"no-duplicate-type-constituents",
	"no-for-in-array",
	"no-implied-eval",
	"no-meaningless-void-operator",
	"no-misused-spread",
	"no-mixed-enums",
	"no-redundant-type-constituents",
	"no-unnecessary-boolean-literal-compare",
	"no-unnecessary-condition",
	"no-unnecessary-qualifier",
	"no-unnecessary-template-expression",
	"no-unnecessary-type-arguments",
	"no-unnecessary-type-assertion",
	"no-unnecessary-type-conversion",
	"no-unnecessary-type-parameters",
	"no-unsafe-argument",
	"no-unsafe-call",
	"no-unsafe-enum-comparison",
	"no-unsafe-member-access",
	"no-unsafe-return",
	"no-unsafe-type-assertion",
	"no-unsafe-unary-minus",
	"no-useless-default-assignment",
	"non-nullable-type-assertion-style",
	"only-throw-error",
	"prefer-destructuring",
	"prefer-find",
	"prefer-includes",
	"prefer-nullish-coalescing",
	"prefer-optional-chain",
	"prefer-promise-reject-errors",
	"prefer-readonly",
	"prefer-readonly-parameter-types",
	"prefer-reduce-type-parameter",
	"prefer-regexp-exec",
	"prefer-return-this-type",
	"prefer-string-starts-ends-with",
	"promise-function-async",
	"related-getter-setter-pairs",
	"require-array-sort-compare",
	"require-await",
	"restrict-plus-operands",
	"restrict-template-expressions",
	"return-await",
	"strict-void-return",
	"switch-exhaustiveness-check",
	"unbound-method",
	"use-unknown-in-catch-callback-variable",
}

// DefaultSeverity returns the severity at which a known rule fires when
// no configuration overrides it. MVP rules default to error; additional
// rules default to off so users opt in deliberately.
func DefaultSeverity(ruleID string) wrapperlint.Severity {
	for _, id := range MVPRuleIDs {
		if id == ruleID {
			return wrapperlint.SeverityError
		}
	}
	return ""
}

// IsKnown reports whether ruleID names any rule the linter ships
// (including additional, opt-in rules).
func IsKnown(ruleID string) bool {
	for _, id := range MVPRuleIDs {
		if id == ruleID {
			return true
		}
	}
	for _, id := range AdditionalTypeAwareRuleIDs {
		if id == ruleID {
			return true
		}
	}
	return false
}
