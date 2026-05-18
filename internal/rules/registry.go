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
	CategoryA11y        Category = "a11y"
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
	{ID: "default-case-last", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "for-direction", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "getter-return", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "guard-for-in", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-array-delete", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-async-promise-executor", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-children-prop", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-class-assign", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-compare-neg-zero", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-cond-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-const-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-constant-binary-expression", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-constant-condition", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-constant-math-min-max-clamp", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-constructor-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-control-regex", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-debugger", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dupe-args", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dupe-class-members", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-duplicate-private-class-members", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dupe-else-if", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-base-to-string", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dupe-keys", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-duplicate-case", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-duplicate-imports", Category: CategoryStyle, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-empty", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-empty-character-class", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-empty-interface", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-empty-pattern", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-empty-type-parameters", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-evolving-types", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-ex-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-explicit-any", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-extra-boolean-cast", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-extra-non-null-assertion", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-fallthrough", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-flat-map-identity", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-focused-tests", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-for-each", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-func-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-function-assign", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-global-assign", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-global-dirname-filename", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-global-is-finite", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-global-is-nan", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-head-import-in-document", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-sparse-arrays", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-static-only-class", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-string-case-mismatch", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-super-without-extends", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-switch-declarations", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-nested-component-definitions", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-new-native-nonconstructor", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-next-async-client-component", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-nodejs-modules", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-non-null-asserted-optional-chain", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-obj-calls", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-octal-escape", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-process-global", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-precision-loss", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-promise-executor-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-qwik-use-visible-task", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-render-return-value", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-restricted-elements", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-prototype-builtins", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-react-prop-assignments", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-setter-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-shadow-restricted-names", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-skipped-tests", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-template-curly-in-string", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-then-property", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-this-before-super", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-ts-ignore", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-undeclared-dependencies", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-type-only-import-attributes", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-undef", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unexpected-multiline", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unresolved-imports", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unreachable", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unreachable-loop", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unreachable-super", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-negation", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-optional-chaining", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unmodified-loop-condition", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unused-function-parameters", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unused-imports", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unused-labels", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unused-expressions", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unused-private-class-members", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unused-vars", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-use-before-define", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-var", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-void-elements-with-children", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-void-type-return", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-with", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-instanceof-array", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-confusing-void-type", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-namespace-keyword", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-iterable-callback-return", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-alert", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-approximative-numeric-constant", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-array-index-key", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-bitwise-operators", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-catch-assign", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-comment-text", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-console", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-const-enum", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-document-cookie", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-document-import-in-page", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-double-equals", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-duplicate-jsx-props", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-duplicate-test-hooks", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-floating-promises", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-for-in-array", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misleading-character-class", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misplaced-assertion", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misrefactored-shorthand-assign", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-react-forward-ref", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-react-specific-props", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-nonoctal-decimal-escape", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misused-new", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "adjacent-overload-signatures", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misused-promises", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-misused-spread", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-mixed-enums", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-implicit-any-let", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-import-assign", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-initializer-with-definite", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-invalid-builtin-instantiation", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-inner-declarations", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-invalid-regexp", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-irregular-whitespace", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-label-var", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-loss-of-precision", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-self-assign", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-solid-destructured-props", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-self-compare", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-argument", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-assignment", Category: CategoryCorrectness, Recommended: true, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-call", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-declaration-merging", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-enum-comparison", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-member-access", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-finally", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-return", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unsafe-unary-minus", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "only-throw-error", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-promise-reject-errors", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "related-getter-setter-pairs", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "require-array-sort-compare", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "require-atomic-updates", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "require-await", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "strict-void-return", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "switch-exhaustiveness-check", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-exhaustive-dependencies", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-hook-at-top-level", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-image-size", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-import-extensions", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-isnan", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-json-import-attributes", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-jsx-key-in-iterable", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-parse-int-radix", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-qwik-classlist", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-qwik-method-usage", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-qwik-valid-lexical-scope", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-single-js-doc-asterisk", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-unique-element-ids", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-unknown-in-catch-callback-variable", Category: CategoryCorrectness, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-yield", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-private-imports", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-vue-data-object-declaration", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-vue-duplicate-keys", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-vue-reserved-keys", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-vue-reserved-props", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-vue-setup-props-reactivity-loss", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
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
	{ID: "no-blank-target", Category: CategorySecurity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dangerously-set-inner-html", Category: CategorySecurity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dangerously-set-inner-html-with-children", Category: CategorySecurity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-global-eval", Category: CategorySecurity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-secrets", Category: CategorySecurity, Fix: FixNone, Stability: StabilityStable},

	// performance — known-slow patterns with a faster equivalent.
	{ID: "no-await-in-loop", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-find", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-includes", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-regexp-exec", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-string-starts-ends-with", Category: CategoryPerformance, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-accumulating-spread", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-barrel-file", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-delete", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-dynamic-namespace-import-access", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-img-element", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-namespace-import", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-re-export-all", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-unwanted-polyfillio", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-google-font-preconnect", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-solid-for-component", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-top-level-regex", Category: CategoryPerformance, Fix: FixNone, Stability: StabilityStable},

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
	{ID: "no-useless-backreference", Category: CategoryCorrectness, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-catch", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-catch-binding", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-continue", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-default-assignment", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-empty-export", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-escape-in-string", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-label", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-rename", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-string-concat", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-string-raw", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-switch-case", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-ternary", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-useless-undefined-initialization", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "non-nullable-type-assertion-style", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-destructuring", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-nullish-coalescing", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-optional-chain", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-reduce-type-parameter", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-return-this-type", Category: CategoryComplexity, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-adjacent-spaces-in-regex", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-arguments", Category: CategoryComplexity, Fix: FixNone, Stability: StabilityStable},

	// style — formatting, naming, ordering. Pure preference.
	{ID: "consistent-type-exports", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "dot-notation", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "naming-convention", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-meaningless-void-operator", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-readonly", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "prefer-readonly-parameter-types", Category: CategoryStyle, RequiresTypeChecking: true, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-self-closing-elements", Category: CategoryStyle, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-single-var-declarator", Category: CategoryStyle, Fix: FixNone, Stability: StabilityStable},

	// a11y — JSX accessibility. Syntactic; no type checker required.
	// Ported from eslint-plugin-jsx-a11y and biome's a11y group.
	{ID: "no-access-key", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-aria-hidden-on-focusable", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-aria-unsupported-elements", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-autofocus", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-distracting-elements", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-header-scope", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-interactive-element-to-noninteractive-role", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-label-without-control", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-noninteractive-element-interactions", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-noninteractive-element-to-interactive-role", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-noninteractive-tabindex", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-positive-tabindex", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-redundant-alt", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-redundant-roles", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-static-element-interactions", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-suspicious-semicolon-in-jsx", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-await", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-error-message", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "no-svg-without-title", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-alt-text", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-anchor-content", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-aria-activedescendant-with-tabindex", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-aria-props-for-role", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-aria-props-supported-by-role", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-button-type", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-focusable-interactive", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-google-font-display", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-number-to-fixed-digits-argument", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-heading-content", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-html-lang", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-iframe-title", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-key-with-click-events", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-key-with-mouse-events", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-media-caption", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-semantic-elements", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-static-response-methods", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-strict-mode", Category: CategorySuspicious, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-valid-anchor", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-valid-aria-props", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-valid-aria-role", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-valid-aria-values", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-valid-autocomplete", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
	{ID: "use-valid-lang", Category: CategoryA11y, Fix: FixNone, Stability: StabilityStable},
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
