package rules

import (
	"sort"
	"testing"

	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// recommendedRules is the set jetlint ships at error severity. Tests
// assert this list is what RecommendedIDs returns and what MVPRuleIDs
// preserves, so changes to the default-on set are visible in diffs.
// Chosen to match both typescript-eslint's `recommendedTypeChecked`
// and oxc's `correctness` category where they agree.
var recommendedRules = []string{
	"await-thenable",
	"no-base-to-string",
	"no-floating-promises",
	"no-misused-promises",
	"no-unsafe-assignment",
}

func TestAllReturnsEveryShippedRule(t *testing.T) {
	got := map[string]bool{}
	for _, m := range All {
		got[m.ID] = true
	}
	want := append([]string{}, recommendedRules...)
	want = append(want, additionalRulesSnapshot()...)
	for _, id := range want {
		if !got[id] {
			t.Errorf("All is missing rule %q", id)
		}
	}
	if len(All) != len(want) {
		t.Errorf("All has %d entries; expected %d (recommended + additional)", len(All), len(want))
	}
}

func TestAllAssignsExactlyOneCategoryPerRule(t *testing.T) {
	validCategories := map[Category]bool{
		CategoryCorrectness: true,
		CategorySuspicious:  true,
		CategorySecurity:    true,
		CategoryPerformance: true,
		CategoryComplexity:  true,
		CategoryStyle:       true,
		CategoryA11y:        true,
		CategoryNursery:     true,
	}
	for _, m := range All {
		if m.Category == "" {
			t.Errorf("rule %q has no category", m.ID)
			continue
		}
		if !validCategories[m.Category] {
			t.Errorf("rule %q has unknown category %q", m.ID, m.Category)
		}
	}
}

func TestRecommendedIDsReturnsTheMVPSet(t *testing.T) {
	got := append([]string{}, RecommendedIDs()...)
	sort.Strings(got)
	want := append([]string{}, recommendedRules...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("RecommendedIDs returned %d ids; expected %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("RecommendedIDs[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestCategoryOfReturnsAssignedCategory(t *testing.T) {
	cases := map[string]Category{
		"no-floating-promises":  CategoryCorrectness,
		"prefer-includes":       CategoryPerformance,
		"no-implied-eval":       CategorySecurity,
		"prefer-optional-chain": CategoryComplexity,
		"dot-notation":          CategoryStyle,
		"return-await":          CategorySuspicious,
	}
	for id, want := range cases {
		got, ok := CategoryOf(id)
		if !ok {
			t.Errorf("CategoryOf(%q): rule not found", id)
			continue
		}
		if got != want {
			t.Errorf("CategoryOf(%q) = %q; want %q", id, got, want)
		}
	}
}

func TestCategoryOfReturnsFalseForUnknownRule(t *testing.T) {
	if _, ok := CategoryOf("not-a-real-rule"); ok {
		t.Errorf("CategoryOf(unknown) returned ok=true; want false")
	}
}

func TestByCategoryReturnsEveryRuleInGroup(t *testing.T) {
	for _, m := range All {
		members := ByCategory(m.Category)
		found := false
		for _, x := range members {
			if x.ID == m.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ByCategory(%q) is missing %q", m.Category, m.ID)
		}
	}
}

func TestIsKnownReturnsTrueForRegisteredRule(t *testing.T) {
	for _, m := range All {
		if !IsKnown(m.ID) {
			t.Errorf("IsKnown(%q) = false; want true", m.ID)
		}
	}
}

func TestIsKnownReturnsFalseForUnknownRule(t *testing.T) {
	if IsKnown("not-a-real-rule") {
		t.Errorf("IsKnown(unknown) = true; want false")
	}
}

func TestDefaultSeverityIsErrorForRecommendedRule(t *testing.T) {
	for _, id := range recommendedRules {
		if got := DefaultSeverity(id); got != wrapperlint.SeverityError {
			t.Errorf("DefaultSeverity(%q) = %q; want %q", id, got, wrapperlint.SeverityError)
		}
	}
}

func TestDefaultSeverityIsEmptyForNonRecommendedRule(t *testing.T) {
	for _, m := range All {
		if m.Recommended {
			continue
		}
		if got := DefaultSeverity(m.ID); got != "" {
			t.Errorf("DefaultSeverity(%q) = %q; want empty", m.ID, got)
		}
	}
}

func TestMVPRuleIDsIsDerivedFromRecommendedFlag(t *testing.T) {
	got := append([]string{}, MVPRuleIDs...)
	sort.Strings(got)
	want := append([]string{}, recommendedRules...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("MVPRuleIDs has %d entries; want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("MVPRuleIDs[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestAdditionalTypeAwareRuleIDsIsDerivedFromNonRecommended(t *testing.T) {
	got := append([]string{}, AdditionalTypeAwareRuleIDs...)
	sort.Strings(got)
	want := additionalRulesSnapshot()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("AdditionalTypeAwareRuleIDs has %d entries; want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("AdditionalTypeAwareRuleIDs[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

// additionalRulesSnapshot is the canonical list of rules that ship beyond
// the recommended/MVP set. Snapshotted in the test so a missed entry in
// All is caught with a precise diff rather than just a length mismatch.
func additionalRulesSnapshot() []string {
	return []string{
		"array-callback-return",
		"consistent-return",
		"consistent-type-exports",
		"constructor-super",
		"default-case-last",
		"dot-notation",
		"for-direction",
		"getter-return",
		"guard-for-in",
		"naming-convention",
		"no-accumulating-spread",
		"no-array-delete",
		"no-async-promise-executor",
		"no-await-in-loop",
		"no-barrel-file",
		"no-blank-target",
		"no-children-prop",
		"no-class-assign",
		"no-compare-neg-zero",
		"no-cond-assign",
		"no-confusing-void-expression",
		"no-const-assign",
		"no-constant-binary-expression",
		"no-constant-condition",
		"no-constant-math-min-max-clamp",
		"no-constructor-return",
		"no-control-regex",
		"no-dangerously-set-inner-html",
		"no-dangerously-set-inner-html-with-children",
		"no-debugger",
		"no-delete",
		"no-deprecated",
		"no-dupe-args",
		"no-dupe-class-members",
		"no-dupe-else-if",
		"no-dupe-keys",
		"no-duplicate-case",
		"no-duplicate-private-class-members",
		"no-duplicate-imports",
		"no-duplicate-type-constituents",
		"no-dynamic-namespace-import-access",
		"no-empty",
		"no-empty-character-class",
		"no-empty-interface",
		"no-empty-pattern",
		"no-evolving-types",
		"no-ex-assign",
		"no-explicit-any",
		"no-extra-non-null-assertion",
		"no-fallthrough",
		"no-focused-tests",
		"no-func-assign",
		"no-function-assign",
		"no-global-assign",
		"no-global-dirname-filename",
		"no-global-eval",
		"no-global-is-finite",
		"no-global-is-nan",
		"no-for-in-array",
		"no-img-element",
		"no-implicit-any-let",
		"no-implied-eval",
		"no-import-assign",
		"no-initializer-with-definite",
		"no-inner-declarations",
		"no-invalid-builtin-instantiation",
		"no-invalid-regexp",
		"no-irregular-whitespace",
		"no-label-var",
		"no-loss-of-precision",
		"no-meaningless-void-operator",
		"no-misleading-character-class",
		"no-misplaced-assertion",
		"no-misrefactored-shorthand-assign",
		"no-misused-spread",
		"no-mixed-enums",
		"no-namespace-import",
		"no-nested-component-definitions",
		"no-new-native-nonconstructor",
		"no-next-async-client-component",
		"no-nodejs-modules",
		"no-non-null-asserted-optional-chain",
		"no-nonoctal-decimal-escape",
		"no-obj-calls",
		"no-octal-escape",
		"no-precision-loss",
		"no-private-imports",
		"no-process-global",
		"no-promise-executor-return",
		"no-qwik-use-visible-task",
		"no-re-export-all",
		"no-render-return-value",
		"no-restricted-elements",
		"no-prototype-builtins",
		"no-react-prop-assignments",
		"no-redundant-type-constituents",
		"no-secrets",
		"no-self-assign",
		"no-self-compare",
		"no-solid-destructured-props",
		"no-setter-return",
		"no-shadow-restricted-names",
		"no-skipped-tests",
		"no-sparse-arrays",
		"no-string-case-mismatch",
		"no-super-without-extends",
		"no-switch-declarations",
		"no-template-curly-in-string",
		"no-then-property",
		"no-this-before-super",
		"no-ts-ignore",
		"no-type-only-import-attributes",
		"no-undeclared-dependencies",
		"no-undef",
		"no-unexpected-multiline",
		"no-unresolved-imports",
		"no-unreachable",
		"no-unreachable-loop",
		"no-unreachable-super",
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
		"no-unsafe-declaration-merging",
		"no-unsafe-enum-comparison",
		"no-unsafe-finally",
		"no-unsafe-member-access",
		"no-unsafe-return",
		"no-unsafe-type-assertion",
		"no-unsafe-negation",
		"no-unsafe-optional-chaining",
		"no-unsafe-unary-minus",
		"no-unmodified-loop-condition",
		"no-unwanted-polyfillio",
		"no-unused-expressions",
		"no-unused-function-parameters",
		"no-unused-imports",
		"no-unused-labels",
		"no-unused-private-class-members",
		"no-unused-vars",
		"no-use-before-define",
		"no-useless-backreference",
		"no-useless-escape-in-string",
		"no-var",
		"no-void-elements-with-children",
		"no-void-type-return",
		"no-with",
		"no-instanceof-array",
		"no-confusing-void-type",
		"no-misused-new",
		"adjacent-overload-signatures",
		"prefer-namespace-keyword",
		"use-iterable-callback-return",
		"no-alert",
		"no-array-index-key",
		"no-bitwise-operators",
		"no-catch-assign",
		"no-comment-text",
		"no-console",
		"no-const-enum",
		"no-document-cookie",
		"no-document-import-in-page",
		"no-double-equals",
		"no-duplicate-jsx-props",
		"no-duplicate-test-hooks",
		"no-head-import-in-document",
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
		"require-atomic-updates",
		"require-await",
		"restrict-plus-operands",
		"restrict-template-expressions",
		"return-await",
		"strict-boolean-expressions",
		"strict-void-return",
		"switch-exhaustiveness-check",
		"unbound-method",
		"use-exhaustive-dependencies",
		"use-hook-at-top-level",
		"use-image-size",
		"use-import-extensions",
		"use-google-font-preconnect",
		"use-isnan",
		"use-json-import-attributes",
		"use-jsx-key-in-iterable",
		"use-parse-int-radix",
		"use-qwik-classlist",
		"use-qwik-method-usage",
		"use-qwik-valid-lexical-scope",
		"use-self-closing-elements",
		"use-single-js-doc-asterisk",
		"use-solid-for-component",
		"use-top-level-regex",
		"use-unique-element-ids",
		"use-unknown-in-catch-callback-variable",
		"no-vue-data-object-declaration",
		"no-vue-duplicate-keys",
		"no-vue-reserved-keys",
		"no-vue-reserved-props",
		"no-vue-setup-props-reactivity-loss",
		"use-yield",
		"valid-typeof",
		// a11y
		"no-access-key",
		"no-aria-hidden-on-focusable",
		"no-aria-unsupported-elements",
		"no-autofocus",
		"no-distracting-elements",
		"no-header-scope",
		"no-interactive-element-to-noninteractive-role",
		"no-label-without-control",
		"no-noninteractive-element-interactions",
		"no-noninteractive-element-to-interactive-role",
		"no-noninteractive-tabindex",
		"no-positive-tabindex",
		"no-redundant-alt",
		"no-redundant-roles",
		"no-static-element-interactions",
		"no-suspicious-semicolon-in-jsx",
		"no-svg-without-title",
		"use-alt-text",
		"use-anchor-content",
		"use-aria-activedescendant-with-tabindex",
		"use-aria-props-for-role",
		"use-aria-props-supported-by-role",
		"use-button-type",
		"use-focusable-interactive",
		"use-heading-content",
		"use-html-lang",
		"use-iframe-title",
		"use-key-with-click-events",
		"use-key-with-mouse-events",
		"use-media-caption",
		"use-semantic-elements",
		"use-valid-anchor",
		"use-valid-aria-props",
		"use-valid-aria-role",
		"use-valid-aria-values",
		"use-valid-autocomplete",
		"use-valid-lang",
	}
}
