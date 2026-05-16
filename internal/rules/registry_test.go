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
		"dot-notation",
		"for-direction",
		"getter-return",
		"naming-convention",
		"no-array-delete",
		"no-async-promise-executor",
		"no-await-in-loop",
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
		"no-debugger",
		"no-deprecated",
		"no-dupe-args",
		"no-dupe-class-members",
		"no-dupe-else-if",
		"no-dupe-keys",
		"no-duplicate-case",
		"no-duplicate-imports",
		"no-duplicate-type-constituents",
		"no-empty-character-class",
		"no-empty-pattern",
		"no-ex-assign",
		"no-fallthrough",
		"no-func-assign",
		"no-global-dirname-filename",
		"no-for-in-array",
		"no-implied-eval",
		"no-import-assign",
		"no-inner-declarations",
		"no-invalid-builtin-instantiation",
		"no-invalid-regexp",
		"no-irregular-whitespace",
		"no-loss-of-precision",
		"no-meaningless-void-operator",
		"no-misleading-character-class",
		"no-misused-spread",
		"no-mixed-enums",
		"no-new-native-nonconstructor",
		"no-nodejs-modules",
		"no-nonoctal-decimal-escape",
		"no-obj-calls",
		"no-process-global",
		"no-promise-executor-return",
		"no-render-return-value",
		"no-prototype-builtins",
		"no-react-prop-assignments",
		"no-redundant-type-constituents",
		"no-self-assign",
		"no-self-compare",
		"no-setter-return",
		"no-sparse-arrays",
		"no-string-case-mismatch",
		"no-switch-declarations",
		"no-template-curly-in-string",
		"no-this-before-super",
		"no-undef",
		"no-unexpected-multiline",
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
		"no-unsafe-enum-comparison",
		"no-unsafe-finally",
		"no-unsafe-member-access",
		"no-unsafe-return",
		"no-unsafe-type-assertion",
		"no-unsafe-negation",
		"no-unsafe-optional-chaining",
		"no-unsafe-unary-minus",
		"no-unmodified-loop-condition",
		"no-unused-function-parameters",
		"no-unused-imports",
		"no-unused-labels",
		"no-unused-private-class-members",
		"no-unused-vars",
		"no-use-before-define",
		"no-useless-backreference",
		"no-void-elements-with-children",
		"no-void-type-return",
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
		"use-isnan",
		"use-json-import-attributes",
		"use-parse-int-radix",
		"use-single-js-doc-asterisk",
		"use-unknown-in-catch-callback-variable",
		"use-yield",
		"valid-typeof",
	}
}
