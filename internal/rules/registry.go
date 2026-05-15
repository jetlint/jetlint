// Package rules is the registry of rules the linter ships. Every rule
// is described once in [All] as a [Metadata] entry; the helpers below —
// [MVPRuleIDs], [AdditionalTypeAwareRuleIDs], [DefaultSeverity],
// [IsKnown], [CategoryOf], [ByCategory], [RecommendedIDs] — derive
// from that single source of truth.
//
// Rule logic lives in subpackages added as each rule lands; this file
// only carries identifiers and metadata so configuration validation
// and CLI flag handling can work without importing each rule
// implementation.
package rules

import (
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// Category groups rules by the kind of problem they flag. See
// docs/RULE-CATEGORIES.md for the definition of each group and the
// decision rubric used to assign rules.
type Category string

const (
	CategoryCorrectness Category = "correctness"
	CategorySuspicious  Category = "suspicious"
	CategorySecurity    Category = "security"
	CategoryPerformance Category = "performance"
	CategoryComplexity  Category = "complexity"
	CategoryStyle       Category = "style"
	CategoryNursery     Category = "nursery"
)

// FixSafety describes whether a rule's autofix preserves semantics.
// Currently no jetlint rule ships an autofix; the field is in place so
// future rules can declare their fix posture without another schema
// change.
type FixSafety string

const (
	FixNone   FixSafety = "none"
	FixSafe   FixSafety = "safe"
	FixUnsafe FixSafety = "unsafe"
)

// Stability lets a rule signal that it is still iterating or has been
// deprecated without changing the rule's Category. A nursery rule may
// live in any category but should not be relied on across releases.
type Stability string

const (
	StabilityStable     Stability = "stable"
	StabilityNursery    Stability = "nursery"
	StabilityDeprecated Stability = "deprecated"
)

// Metadata is everything the registry tracks about a single rule.
// The ID is the kebab-case identifier users put in .jetlintrc.json.
type Metadata struct {
	ID                   string
	Category             Category
	Recommended          bool
	RequiresTypeChecking bool
	Fix                  FixSafety
	Stability            Stability
}

// All is the single source of truth for every rule jetlint ships.
// Entries are listed alphabetically by ID within each category to keep
// diffs small when rules are added or moved between groups.
var All = []Metadata{
	// correctness — code that is wrong: runtime bugs, undefined behavior,
	// type holes. No legitimate reason to write this.
	{ID: "array-callback-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "await-thenable", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "consistent-return", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "constructor-super", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "for-direction", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "getter-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-array-delete", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-async-promise-executor", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-class-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-compare-neg-zero", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-cond-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-const-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-constant-condition", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-constructor-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-debugger", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dupe-class-members", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dupe-else-if", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-base-to-string", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dupe-keys", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-duplicate-case", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-duplicate-imports", Category: CategoryStyle, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-empty-pattern", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-ex-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-fallthrough", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-func-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-sparse-arrays", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-new-native-nonconstructor", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-obj-calls", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-promise-executor-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-prototype-builtins", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-setter-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-template-curly-in-string", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-this-before-super", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-undef", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unexpected-multiline", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unreachable", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-negation", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-optional-chaining", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unused-private-class-members", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-use-before-define", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-floating-promises", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-for-in-array", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misused-promises", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misused-spread", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-mixed-enums", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-import-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-inner-declarations", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-irregular-whitespace", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-loss-of-precision", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-self-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-self-compare", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-argument", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-assignment", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-call", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-enum-comparison", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-member-access", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-finally", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-return", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-unary-minus", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "only-throw-error", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-promise-reject-errors", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "related-getter-setter-pairs", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "require-array-sort-compare", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "require-await", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "strict-void-return", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "switch-exhaustiveness-check", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-isnan", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-unknown-in-catch-callback-variable", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "valid-typeof", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},

	// suspicious — usually wrong, occasionally intentional.
	{ID: "no-confusing-void-expression", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-deprecated", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-type-assertion", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "promise-function-async", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "restrict-plus-operands", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "restrict-template-expressions", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "return-await", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "strict-boolean-expressions", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "unbound-method", Category: CategorySuspicious, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},

	// security — injection, eval, prototype pollution, unsafe deserialization.
	{ID: "no-implied-eval", Category: CategorySecurity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},

	// performance — known-slow patterns with a faster equivalent.
	{ID: "no-await-in-loop", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-find", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-includes", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-regexp-exec", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-string-starts-ends-with", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},

	// complexity — needless complication with a simpler equivalent.
	{ID: "no-duplicate-type-constituents", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-redundant-type-constituents", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-boolean-literal-compare", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-condition", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-qualifier", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-template-expression", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-type-arguments", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-type-assertion", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-type-conversion", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unnecessary-type-parameters", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-default-assignment", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "non-nullable-type-assertion-style", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-destructuring", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-nullish-coalescing", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-optional-chain", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-reduce-type-parameter", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-return-this-type", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},

	// style — formatting, naming, ordering. Pure preference.
	{ID: "consistent-type-exports", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "dot-notation", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "naming-convention", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-meaningless-void-operator", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-readonly", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-readonly-parameter-types", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
}

// MVPRuleIDs preserves the legacy ordering of the recommended set so
// callers that show "known rules" in error messages keep their existing
// output order. New code should prefer [RecommendedIDs].
var MVPRuleIDs = mvpRuleIDsInOrder()

// AdditionalTypeAwareRuleIDs lists every rule that is shipped but not
// recommended-on-by-default, in the order existing callers expect
// (alphabetical by ID). Preserved for backward compatibility with
// configuration validation and CLI handling that loops over the
// off-by-default rule set.
var AdditionalTypeAwareRuleIDs = additionalRuleIDsInOrder()

// DefaultSeverity returns the severity at which a known rule fires when
// no configuration overrides it. Recommended rules default to error;
// every other rule defaults to off (empty severity).
func DefaultSeverity(ruleID string) wrapperlint.Severity {
	for _, m := range All {
		if m.ID == ruleID && m.Recommended {
			return wrapperlint.SeverityError
		}
	}
	return ""
}

// IsKnown reports whether ruleID names any rule the linter ships.
func IsKnown(ruleID string) bool {
	for _, m := range All {
		if m.ID == ruleID {
			return true
		}
	}
	return false
}

// CategoryOf returns the Category assigned to ruleID. The second return
// value is false when no rule with that ID is registered.
func CategoryOf(ruleID string) (Category, bool) {
	for _, m := range All {
		if m.ID == ruleID {
			return m.Category, true
		}
	}
	return "", false
}

// ByCategory returns every rule whose Category equals c, in registry
// order. Returns an empty slice when no rules belong to the category.
func ByCategory(c Category) []Metadata {
	out := []Metadata{}
	for _, m := range All {
		if m.Category == c {
			out = append(out, m)
		}
	}
	return out
}

// RecommendedIDs returns the IDs of every rule jetlint ships with
// Recommended=true. These are the rules in the jetlint:recommended
// preset and the source of the legacy [MVPRuleIDs] ordering.
func RecommendedIDs() []string {
	out := []string{}
	for _, m := range All {
		if m.Recommended {
			out = append(out, m.ID)
		}
	}
	return out
}

// mvpRuleIDsInOrder returns the recommended IDs in the order existing
// callers used before the registry refactor. The set tracks the
// upstream consensus (typescript-eslint's recommendedTypeChecked
// intersected with oxc's correctness category) plus the legacy
// ordering callers depend on.
func mvpRuleIDsInOrder() []string {
	return []string{
		"await-thenable",
		"no-base-to-string",
		"no-floating-promises",
		"no-misused-promises",
		"no-unsafe-assignment",
	}
}

// additionalRuleIDsInOrder returns the non-recommended IDs in the
// alphabetical order existing callers used before the registry
// refactor.
func additionalRuleIDsInOrder() []string {
	mvp := map[string]bool{}
	for _, id := range mvpRuleIDsInOrder() {
		mvp[id] = true
	}
	out := []string{}
	for _, m := range All {
		if !mvp[m.ID] {
			out = append(out, m.ID)
		}
	}
	return out
}
