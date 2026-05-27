# Rule categorization

jetlint groups every rule into exactly one **category**, paired with a small
set of orthogonal flags. The goal is the same one Biome and Clippy achieve:
let users enable a whole class of rules without learning each individual
rule, while keeping a clean mental model.

## The seven groups

| Group           | Definition                                                                                       | Default severity |
| --------------- | ------------------------------------------------------------------------------------------------ | ---------------- |
| **correctness** | Code that is wrong: runtime bugs, undefined behavior, type holes. No legitimate reason to write. | error            |
| **suspicious**  | Code that smells. Usually wrong, occasionally intentional. Author should justify or fix.         | warn             |
| **security**    | Patterns enabling injection, eval, prototype pollution, unsafe deserialization.                  | error            |
| **performance** | Known-slow patterns with a faster equivalent. No correctness impact.                             | warn             |
| **complexity**  | Needless complication with a simpler equivalent. No correctness or perf impact.                  | warn             |
| **a11y**        | JSX accessibility rules: ARIA props, semantic elements, keyboard interactions.                   | off (opt-in)     |
| **style**       | Formatting, naming, ordering. Pure preference; team-configurable.                                | off (opt-in)     |
| **nursery**     | New or iterating rules. May change shape or move to another group. Not in `recommended`.         | off              |

Deliberately omitted (for now):

- **pedantic** / **restriction** (Clippy-style) — opinionated or contradictory
  rules. Fold them into `style` and ship off-by-default until a real need
  appears.

## Decision rubric

Apply in order; the first match wins:

1. **Wrong at runtime, or definitely a bug?** → `correctness`
2. **Enables injection, eval, prototype pollution, unsafe deserialization?** → `security`
3. **Pure perf cost with a known fast alternative?** → `performance`
4. **JSX accessibility?** → `a11y`
5. **Smells, but a thoughtful developer could mean it?** → `suspicious`
6. **Simpler equivalent exists, no perf or correctness impact?** → `complexity`
7. **Formatting, naming, or ordering only?** → `style`
8. **Still iterating? Might break?** → `nursery` (overrides the above)

Tie-breaker for cross-cutting rules — e.g. `no-for-in-array` is both buggy and
slow: pick the **highest-severity framing**. Correctness beats performance
beats complexity.

## Orthogonal flags

Every rule carries these in addition to its group:

| Flag                   | Values                                  | Purpose                                                                       |
| ---------------------- | --------------------------------------- | ----------------------------------------------------------------------------- |
| `recommended`          | bool                                    | Included in the `jetlint:recommended` preset.                                 |
| `requiresTypeChecking` | bool                                    | Rule needs the type checker. Surfaces cost: program load only runs if needed. |
| `fix`                  | `none` \| `safe` \| `unsafe`            | `safe` always preserves semantics; `unsafe` needs review.                     |
| `stability`            | `stable` \| `nursery` \| `deprecated`   | Lets a stable rule be deprecated without moving group.                        |

`stability = nursery` and `group = nursery` overlap but aren't redundant: the
group controls discovery and default severity; the flag lets a rule graduate
from the nursery into (say) `correctness` while still being marked nursery
for one release as a signal to early adopters.

## Current rule assignments

**373 rules are registered today.** 65 are type-aware (they query the
TypeScript checker), 5 are `recommended` (on by default), and 6 are still
scaffolded no-ops that register but emit nothing. The canonical assignment
lives in [`internal/rules/registry.go`](../internal/rules/registry.go); the
lists below are generated from it. Scaffolded stubs are marked `*`.

| Category    | Rules | Type-aware |
| ----------- | ----- | ---------- |
| correctness | 105   | 24         |
| suspicious  | 97    | 13         |
| style       | 61    | 6          |
| complexity  | 52    | 17         |
| a11y        | 36    | 0          |
| performance | 16    | 4          |
| security    | 6     | 1          |
| nursery     | 0     | 0          |

### correctness (105, 24 type-aware)

`array-callback-return`, `await-thenable`, `consistent-return`,
`constructor-super`, `for-direction`, `no-array-delete`, `no-base-to-string`,
`no-children-prop`, `no-cond-assign`, `no-const-assign`, `no-constant-condition`,
`no-constant-math-min-max-clamp`, `no-constructor-return`,
`no-duplicate-private-class-members`, `no-empty-character-class`,
`no-empty-pattern`, `no-ex-assign`, `no-floating-promises`, `no-for-in-array`,
`no-func-assign`, `no-global-dirname-filename`, `no-initializer-with-definite`,
`no-inner-declarations`, `no-invalid-builtin-instantiation`, `no-invalid-regexp`,
`no-loss-of-precision`, `no-misused-promises`, `no-misused-spread`,
`no-mixed-enums`, `no-nested-component-definitions`, `no-new-native-nonconstructor`,
`no-next-async-client-component`, `no-nodejs-modules`, `no-nonoctal-decimal-escape`,
`no-obj-calls`, `no-precision-loss`, `no-private-imports`, `no-process-global`,
`no-promise-executor-return`, `no-qwik-use-visible-task`, `no-react-prop-assignments`,
`no-render-return-value`, `no-restricted-elements`, `no-self-assign`,
`no-setter-return`, `no-solid-destructured-props`, `no-string-case-mismatch`,
`no-super-without-extends`, `no-switch-declarations`, `no-this-before-super`,
`no-type-only-import-attributes`, `no-undeclared-dependencies`, `no-undef`,
`no-unmodified-loop-condition`, `no-unreachable`, `no-unreachable-loop`,
`no-unreachable-super`, `no-unresolved-imports`, `no-unsafe-argument`,
`no-unsafe-assignment`, `no-unsafe-call`, `no-unsafe-enum-comparison`,
`no-unsafe-finally`, `no-unsafe-member-access`, `no-unsafe-optional-chaining`,
`no-unsafe-return`, `no-unsafe-unary-minus`, `no-unused-function-parameters`,
`no-unused-imports`, `no-unused-labels`, `no-unused-private-class-members`,
`no-unused-vars`, `no-use-before-define`, `no-useless-backreference`,
`no-void-elements-with-children`, `no-void-type-return`,
`no-vue-data-object-declaration`, `no-vue-duplicate-keys`, `no-vue-reserved-keys`,
`no-vue-reserved-props`, `no-vue-setup-props-reactivity-loss`, `only-throw-error`,
`prefer-promise-reject-errors`, `related-getter-setter-pairs`,
`require-array-sort-compare`, `require-atomic-updates`, `require-await`,
`strict-void-return`, `switch-exhaustiveness-check`, `use-exhaustive-dependencies`,
`use-hook-at-top-level`, `use-image-size`, `use-import-extensions`, `use-isnan`,
`use-json-import-attributes`, `use-jsx-key-in-iterable`, `use-parse-int-radix`,
`use-qwik-classlist`, `use-qwik-method-usage`, `use-qwik-valid-lexical-scope`,
`use-single-js-doc-asterisk`, `use-unique-element-ids`,
`use-unknown-in-catch-callback-variable`, `use-yield`, `valid-typeof`

Non-type-aware correctness rules carry `RequiresTypeChecking: false`: they walk
the AST without ever calling the checker and rely on syntactic helpers like
[Node.SourceText][srctext], [Node.PropertyAccessName][pan], and
[Node.LiteralText][lit].

[srctext]: https://pkg.go.dev/github.com/microsoft/typescript-go/pkg/checker#Node.SourceText
[pan]: https://pkg.go.dev/github.com/microsoft/typescript-go/pkg/checker#Node.PropertyAccessName
[lit]: https://pkg.go.dev/github.com/microsoft/typescript-go/pkg/checker#Node.LiteralText

### suspicious (97, 13 type-aware)

`adjacent-overload-signatures`, `default-case-last`, `getter-return`,
`guard-for-in`, `no-alert`, `no-approximative-numeric-constant`,
`no-array-index-key`, `no-assign-in-expressions`, `no-async-promise-executor`,
`no-bitwise-operators`, `no-catch-assign`, `no-class-assign`, `no-comment-text`,
`no-compare-neg-zero`, `no-confusing-labels`, `no-confusing-void-expression`,
`no-confusing-void-type`, `no-console`, `no-const-enum`,
`no-constant-binary-expression`, `no-control-regex`, `no-debugger`,
`no-deprecated`, `no-deprecated-imports`, `no-document-cookie`,
`no-document-import-in-page`, `no-double-equals`, `no-dupe-args`,
`no-dupe-class-members`, `no-dupe-else-if`, `no-dupe-keys`, `no-duplicate-case`,
`no-duplicate-jsx-props`, `no-duplicate-test-hooks`, `no-empty`,
`no-empty-interface`, `no-empty-source`, `no-evolving-types`, `no-explicit-any`,
`no-exports-in-test`, `no-extra-non-null-assertion`, `no-fallthrough`,
`no-focused-tests`, `no-function-assign`, `no-global-assign`, `no-global-is-finite`,
`no-global-is-nan`, `no-head-import-in-document`*, `no-implicit-any-let`,
`no-import-assign`, `no-import-cycles`, `no-instanceof-array`,
`no-irregular-whitespace`, `no-label-var`, `no-misleading-character-class`,
`no-misplaced-assertion`, `no-misrefactored-shorthand-assign`, `no-misused-new`,
`no-non-null-asserted-optional-chain`, `no-octal-escape`, `no-prototype-builtins`,
`no-react-forward-ref`, `no-react-specific-props`, `no-redeclare`,
`no-redundant-use-strict`, `no-self-compare`, `no-shadow-restricted-names`,
`no-skipped-tests`, `no-sparse-arrays`, `no-suspicious-semicolon-in-jsx`,
`no-template-curly-in-string`, `no-then-property`, `no-ts-ignore`,
`no-unassigned-variables`, `no-unexpected-multiline`,
`no-unsafe-declaration-merging`, `no-unsafe-negation`, `no-unsafe-type-assertion`,
`no-unused-expressions`, `no-useless-escape-in-string`, `no-useless-regex-backrefs`,
`no-var`, `no-with`, `prefer-namespace-keyword`, `promise-function-async`,
`restrict-plus-operands`, `restrict-template-expressions`, `return-await`,
`strict-boolean-expressions`, `unbound-method`, `use-await`, `use-error-message`,
`use-google-font-display`, `use-iterable-callback-return`,
`use-number-to-fixed-digits-argument`, `use-static-response-methods`,
`use-strict-mode`

### security (6, 1 type-aware)

`no-blank-target`, `no-dangerously-set-inner-html`,
`no-dangerously-set-inner-html-with-children`, `no-global-eval`,
`no-implied-eval`, `no-secrets`

### performance (16, 4 type-aware)

`no-accumulating-spread`, `no-await-in-loop`, `no-barrel-file`, `no-delete`,
`no-dynamic-namespace-import-access`, `no-img-element`, `no-namespace-import`,
`no-re-export-all`, `no-unwanted-polyfillio`, `prefer-find`, `prefer-includes`,
`prefer-regexp-exec`, `prefer-string-starts-ends-with`, `use-google-font-preconnect`,
`use-solid-for-component`, `use-top-level-regex`

### complexity (52, 17 type-aware)

`no-adjacent-spaces-in-regex`, `no-arguments`, `no-comma-operator`,
`no-duplicate-type-constituents`, `no-empty-type-parameters`,
`no-excessive-lines-per-function`, `no-excessive-nested-test-suites`,
`no-extra-boolean-cast`, `no-flat-map-identity`, `no-for-each`,
`no-redundant-type-constituents`, `no-restricted-types`*, `no-static-only-class`,
`no-this-in-static`, `no-unnecessary-boolean-literal-compare`,
`no-unnecessary-condition`, `no-unnecessary-qualifier`,
`no-unnecessary-template-expression`, `no-unnecessary-type-arguments`,
`no-unnecessary-type-assertion`, `no-unnecessary-type-conversion`,
`no-unnecessary-type-parameters`, `no-useless-catch`, `no-useless-catch-binding`,
`no-useless-continue`, `no-useless-default-assignment`, `no-useless-empty-export`,
`no-useless-label`, `no-useless-rename`, `no-useless-string-concat`,
`no-useless-string-raw`, `no-useless-switch-case`, `no-useless-ternary`,
`no-useless-type-constraint`, `no-useless-undefined-initialization`, `no-void`,
`non-nullable-type-assertion-style`, `prefer-destructuring`,
`prefer-nullish-coalescing`, `prefer-optional-chain`, `prefer-reduce-type-parameter`,
`prefer-return-this-type`, `use-arrow-function`, `use-date-now`, `use-flat-map`,
`use-index-of`, `use-literal-keys`, `use-max-params`, `use-numeric-literals`,
`use-simple-number-keys`, `use-simplified-logic-expression`, `use-while`

### a11y (36, 0 type-aware)

`no-access-key`, `no-aria-hidden-on-focusable`, `no-aria-unsupported-elements`,
`no-autofocus`, `no-distracting-elements`, `no-header-scope`,
`no-interactive-element-to-noninteractive-role`, `no-label-without-control`,
`no-noninteractive-element-interactions`,
`no-noninteractive-element-to-interactive-role`, `no-noninteractive-tabindex`,
`no-positive-tabindex`, `no-redundant-alt`, `no-redundant-roles`,
`no-static-element-interactions`, `no-svg-without-title`, `use-alt-text`,
`use-anchor-content`, `use-aria-activedescendant-with-tabindex`,
`use-aria-props-for-role`, `use-aria-props-supported-by-role`, `use-button-type`,
`use-focusable-interactive`, `use-heading-content`, `use-html-lang`,
`use-iframe-title`, `use-key-with-click-events`, `use-key-with-mouse-events`,
`use-media-caption`, `use-semantic-elements`, `use-valid-anchor`,
`use-valid-aria-props`, `use-valid-aria-role`, `use-valid-aria-values`,
`use-valid-autocomplete`, `use-valid-lang`

All a11y rules are JSX-syntactic and carry `RequiresTypeChecking: false`.
Default severity is `off` — opt in via `.jetlintrc.json`. The rules are ported
from `eslint-plugin-jsx-a11y` and Biome's a11y group; each has a passing
`EslintCompatibility`/`BiomeCompatibility` harness against the upstream fixtures
under `biome-fixtures/`.

### style (61, 6 type-aware)

`consistent-type-exports`, `dot-notation`, `naming-convention`, `no-common-js`,
`no-default-export`, `no-done-callback`, `no-duplicate-imports`, `no-enum`,
`no-exported-imports`, `no-head-element`*, `no-implicit-boolean`,
`no-meaningless-void-operator`, `no-namespace`, `no-negation-else`,
`no-nested-ternary`, `no-non-null-assertion`, `no-parameter-assign`,
`no-parameter-properties`, `no-process-env`, `no-restricted-globals`,
`no-restricted-imports`*, `no-shouty-constants`, `no-substr`,
`no-unused-template-literal`, `no-useless-else`, `no-yoda-expression`,
`prefer-readonly`, `prefer-readonly-parameter-types`, `use-array-literals`,
`use-as-const-assertion`, `use-block-statements`, `use-collapsed-else-if`,
`use-collapsed-if`, `use-consistent-array-type`, `use-consistent-arrow-return`,
`use-consistent-builtin-instantiation`, `use-consistent-member-accessibility`*,
`use-consistent-object-definitions`*, `use-consistent-type-definitions`,
`use-default-parameter-last`, `use-enum-initializers`, `use-explicit-length-check`,
`use-exponentiation-operator`, `use-exports-last`, `use-for-of`,
`use-fragment-syntax`, `use-grouped-accessor-pairs`, `use-node-assert-strict`,
`use-nodejs-import-protocol`, `use-number-namespace`, `use-numeric-separators`,
`use-object-spread`, `use-react-function-components`, `use-self-closing-elements`,
`use-shorthand-assign`, `use-shorthand-function-type`, `use-single-var-declarator`,
`use-symbol-description`, `use-throw-new-error`, `use-throw-only-error`,
`use-trim-start-end`

### nursery (0)

None yet. Reserved for new rules during incubation.

### recommended (on by default)

`await-thenable`, `no-base-to-string`, `no-floating-promises`,
`no-misused-promises`, `no-unsafe-assignment`.

### scaffolded stubs (registered, no diagnostics yet)

`no-head-element`, `no-head-import-in-document`, `no-restricted-imports`,
`no-restricted-types`, `use-consistent-member-accessibility`,
`use-consistent-object-definitions`.

## Edge cases worth recording

- **`no-unsafe-*` family** lives in `correctness`, not `suspicious`. An `any`
  leak is a hole in the type system — same severity as a missing `await`.
  typescript-eslint groups these in `strict-type-checked`, which is preset
  metadata, not category metadata; we keep the category orthogonal to the
  preset by leaving `recommended: false` if we want softer adoption.
- **`return-await`** is `suspicious`, not `correctness`. It has real
  correctness implications (stack traces, microtask timing) but most code
  works either way; reasonable people disagree.
- **`strict-boolean-expressions`** is `suspicious`, not `correctness`. Falsy
  coercion is JavaScript's most-used idiom; calling all of it incorrect is a
  stance, not a fact.
- **`no-deprecated`** is `suspicious`, not `correctness`. Using a deprecated
  API isn't broken — just unwise. Authors may be in a migration window.
- **`no-for-in-array`** is `correctness`, despite the perf framing in some
  docs. Iterating string keys over an array is almost always a bug, not just
  a slow choice.

## Beyond type-aware rules

jetlint has already grown well past its type-aware core: of the 373 rules,
308 are syntactic (no checker required), ported from ESLint core,
eslint-plugin-jsx-a11y, and Biome. Most rules once planned here have shipped —
`no-var`, `no-unused-vars`, `no-self-compare`, `no-unreachable`,
`no-constant-condition`, `no-fallthrough`, `no-empty`, and the full 36-rule
a11y group among them.

The remaining gap is **security**: 6 rules today. High-value sources still to
mine include `eslint-plugin-security` (no-new-func, detect-object-injection),
`eslint-plugin-no-secrets`, and node-security rules — security and suspicious
deliver the most value per rule-of-effort because they catch bugs
`tsc --noEmit` and Prettier cannot.

## Configuration surface

Today, `.jetlintrc.json` toggles rules individually:

```json
{
  "rules": {
    "no-floating-promises": "error",
    "prefer-includes": "warn"
  }
}
```

Once categories ship, group-level toggles compose with rule-level ones:

```json
{
  "categories": {
    "correctness": "error",
    "performance": "warn",
    "style": "off"
  },
  "rules": {
    "return-await": "error"
  }
}
```

Resolution order: built-in defaults → category overrides → rule overrides.
Explicit rule severity always wins.
