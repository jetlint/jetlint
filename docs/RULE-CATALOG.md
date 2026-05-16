# Rule catalog

Total rules under consideration: **490**. **118 shipped** in jetlint today; the rest are planned.

This catalog is the planning document for which rules jetlint intends to support and where each lives in the [category taxonomy](RULE-CATEGORIES.md). It is the union of biome's full rule surface (~424 rules across 8 categories, including framework-specific rules and biome's nursery) and jetlint's existing 118 shipped rules. The aim is not to ship everything immediately — it is to make every category Milestone resolvable from a single document.

## How to read this

- **Status**: ✓ shipped (in jetlint's registry today) · ⏳ planned (catalog entry only).
- **Source**: where the rule semantics originate. `eslint`, `typescript-eslint`, `eslint-plugin-react`, `eslint-plugin-vue`, etc. `biome-original` for rules biome wrote without an upstream.
- **Framework tag**: `[react]`, `[vue]`, `[solid]`, `[qwik]`, `[next]`, `[playwright]`, `[drizzle]`, `[react-native]`. Rules without a tag are framework-agnostic. Frameworks are tracked as **issue labels**, not as separate Milestones — see the cross-cutting section at the end.
- **Biome alias**: when the biome name differs from the canonical eslint/ts-eslint name, the biome name is shown in the Notes column.
- **Move marker** (⚠️): the rule currently ships in a different jetlint category than this catalog proposes. The catalog's placement is the proposal; the registry change happens in a follow-up PR.
- **Biome divergence note**: when this catalog places a rule somewhere other than biome (only happens for biome-nursery rules jetlint ships at stable severity), the rule lists biome's nursery placement in the Notes column.

## Placement rubric

This catalog uses biome's category for biome rules (since biome and jetlint share the same eight category names), with the following carve-outs:

1. **Semantic placement wins over biome's nursery.** Biome's `nursery` is an organizational state ("we haven't decided where this lives"), but jetlint already ships 12 of those rules at stable severity, several in the recommended preset. Those rules go in their semantic home in this catalog (correctness, security, performance, or complexity) with a note that biome currently nurseries them. jetlint's `nursery` is reserved for rules whose **behavior** is still iterating, tracked via the `Stability` flag separately from category.
2. **Framework rules are included.** Biome's React/Vue/Solid/Qwik/Next/Playwright/Drizzle/React Native rules ship in their respective categories with a framework tag, even though jetlint has no framework-aware mode today; the tag reserves the placement.
3. **jetlint-only rules** (type-aware rules ported from typescript-eslint that biome doesn't have) keep their current jetlint category.
4. **Disagreements with biome on shared rules** (e.g. biome puts `no-self-compare` in `suspicious` while jetlint has it in `correctness`) follow biome's lead and are flagged with ⚠️ so the registry-move PR is easy to assemble.

## correctness — 101 rules

Code that is wrong: runtime bugs, undefined behavior, type holes. No legitimate reason to write.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `array-callback-return` | ✓ | typescript-eslint |  |  |
| `await-thenable` | ✓ | typescript-eslint |  | biome: `use-await-thenable` · biome category: `nursery` — semantic placement applied here |
| `consistent-return` | ✓ | typescript-eslint |  |  |
| `constructor-super` | ✓ | eslint |  | biome: `no-invalid-constructor-super` |
| `for-direction` | ✓ | eslint |  | biome: `use-valid-for-direction` |
| `no-array-delete` | ✓ | typescript-eslint |  |  |
| `no-base-to-string` | ✓ | typescript-eslint |  | biome category: `nursery` — semantic placement applied here |
| `no-children-prop` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-cond-assign` | ✓ | typescript-eslint |  |  |
| `no-const-assign` | ✓ | eslint |  |  |
| `no-constant-condition` | ✓ | eslint |  | ⚠️ currently shipped in `suspicious` |
| `no-constant-math-min-max-clamp` | ⏳ | eslint |  |  |
| `no-constructor-return` | ✓ | eslint |  |  |
| `no-empty-character-class` | ✓ | eslint |  | biome: `no-empty-character-class-in-regex` |
| `no-empty-pattern` | ✓ | eslint |  |  |
| `no-ex-assign` | ✓ | typescript-eslint |  |  |
| `no-floating-promises` | ✓ | typescript-eslint |  | biome category: `nursery` — semantic placement applied here |
| `no-for-in-array` | ✓ | typescript-eslint |  |  |
| `no-func-assign` | ✓ | typescript-eslint |  |  |
| `no-global-dirname-filename` | ⏳ | biome-original |  |  |
| `no-inner-declarations` | ✓ | eslint |  | ⚠️ currently shipped in `suspicious` |
| `no-invalid-builtin-instantiation` | ⏳ | eslint |  |  |
| `no-invalid-regexp` | ✓ | typescript-eslint |  |  |
| `no-loss-of-precision` | ✓ | typescript-eslint |  |  |
| `no-misused-promises` | ✓ | typescript-eslint |  | biome category: `nursery` — semantic placement applied here |
| `no-misused-spread` | ✓ | typescript-eslint |  |  |
| `no-mixed-enums` | ✓ | typescript-eslint |  |  |
| `no-nested-component-definitions` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-new-native-nonconstructor` | ✓ | typescript-eslint |  |  |
| `no-next-async-client-component` | ⏳ | eslint-plugin-next | [next] |  |
| `no-nodejs-modules` | ⏳ | biome-original |  |  |
| `no-nonoctal-decimal-escape` | ⏳ | eslint |  |  |
| `no-obj-calls` | ✓ | eslint |  | biome: `no-global-object-calls` |
| `no-precision-loss` | ⏳ | eslint |  |  |
| `no-private-imports` | ⏳ | biome-original |  |  |
| `no-process-global` | ⏳ | biome-original |  |  |
| `no-promise-executor-return` | ✓ | typescript-eslint |  |  |
| `no-qwik-use-visible-task` | ⏳ | eslint-plugin-qwik | [qwik] |  |
| `no-react-prop-assignments` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-render-return-value` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-restricted-elements` | ⏳ | biome-original |  |  |
| `no-self-assign` | ✓ | eslint |  |  |
| `no-setter-return` | ✓ | eslint |  |  |
| `no-solid-destructured-props` | ⏳ | eslint-plugin-solid | [solid] |  |
| `no-string-case-mismatch` | ⏳ | eslint |  |  |
| `no-switch-declarations` | ⏳ | eslint |  |  |
| `no-this-before-super` | ✓ | typescript-eslint |  |  |
| `no-undeclared-dependencies` | ⏳ | biome-original |  |  |
| `no-undef` | ✓ | eslint |  | biome: `no-undeclared-variables` |
| `no-unmodified-loop-condition` | ✓ | typescript-eslint |  |  |
| `no-unreachable` | ✓ | eslint |  |  |
| `no-unreachable-loop` | ✓ | typescript-eslint |  |  |
| `no-unreachable-super` | ⏳ | eslint |  |  |
| `no-unresolved-imports` | ⏳ | biome-original |  |  |
| `no-unsafe-argument` | ✓ | typescript-eslint |  |  |
| `no-unsafe-assignment` | ✓ | typescript-eslint |  |  |
| `no-unsafe-call` | ✓ | typescript-eslint |  |  |
| `no-unsafe-enum-comparison` | ✓ | typescript-eslint |  |  |
| `no-unsafe-finally` | ✓ | eslint |  |  |
| `no-unsafe-member-access` | ✓ | typescript-eslint |  |  |
| `no-unsafe-optional-chaining` | ✓ | eslint |  |  |
| `no-unsafe-return` | ✓ | typescript-eslint |  |  |
| `no-unsafe-unary-minus` | ✓ | typescript-eslint |  |  |
| `no-unused-function-parameters` | ⏳ | eslint |  |  |
| `no-unused-imports` | ⏳ | eslint |  |  |
| `no-unused-labels` | ⏳ | eslint |  |  |
| `no-unused-private-class-members` | ✓ | eslint |  |  |
| `no-unused-vars` | ✓ | eslint |  | biome: `no-unused-variables` |
| `no-use-before-define` | ✓ | eslint |  | biome: `no-invalid-use-before-declaration` |
| `no-useless-backreference` | ✓ | typescript-eslint |  |  |
| `no-void-elements-with-children` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-void-type-return` | ⏳ | eslint |  |  |
| `no-vue-data-object-declaration` | ⏳ | eslint-plugin-vue | [vue] |  |
| `no-vue-duplicate-keys` | ⏳ | eslint-plugin-vue | [vue] |  |
| `no-vue-reserved-keys` | ⏳ | eslint-plugin-vue | [vue] |  |
| `no-vue-reserved-props` | ⏳ | eslint-plugin-vue | [vue] |  |
| `no-vue-setup-props-reactivity-loss` | ⏳ | eslint-plugin-vue | [vue] |  |
| `only-throw-error` | ✓ | typescript-eslint |  |  |
| `prefer-promise-reject-errors` | ✓ | typescript-eslint |  |  |
| `related-getter-setter-pairs` | ✓ | typescript-eslint |  |  |
| `require-array-sort-compare` | ✓ | typescript-eslint |  | biome: `use-array-sort-compare` · biome category: `nursery` — semantic placement applied here |
| `require-atomic-updates` | ✓ | typescript-eslint |  |  |
| `require-await` | ✓ | typescript-eslint |  |  |
| `strict-void-return` | ✓ | typescript-eslint |  |  |
| `switch-exhaustiveness-check` | ✓ | typescript-eslint |  |  |
| `use-exhaustive-dependencies` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `use-hook-at-top-level` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `use-image-size` | ⏳ | biome-original |  |  |
| `use-import-extensions` | ⏳ | biome-original |  |  |
| `use-isnan` | ✓ | eslint |  | biome: `use-is-nan` |
| `use-json-import-attributes` | ⏳ | eslint |  |  |
| `use-jsx-key-in-iterable` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `use-parse-int-radix` | ⏳ | eslint |  |  |
| `use-qwik-classlist` | ⏳ | eslint-plugin-qwik | [qwik] |  |
| `use-qwik-method-usage` | ⏳ | eslint-plugin-qwik | [qwik] |  |
| `use-qwik-valid-lexical-scope` | ⏳ | eslint-plugin-qwik | [qwik] |  |
| `use-single-js-doc-asterisk` | ⏳ | eslint |  |  |
| `use-unique-element-ids` | ⏳ | eslint |  |  |
| `use-unknown-in-catch-callback-variable` | ✓ | typescript-eslint |  |  |
| `use-yield` | ⏳ | eslint |  |  |
| `valid-typeof` | ✓ | eslint |  | biome: `use-valid-typeof` |

## suspicious — 97 rules

Code that smells. Usually wrong, occasionally intentional. The author should justify or fix.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `adjacent-overload-signatures` | ⏳ | eslint |  | biome: `use-adjacent-overload-signatures` |
| `default-case-last` | ⏳ | eslint |  | biome: `use-default-switch-clause-last` |
| `getter-return` | ✓ | eslint |  | biome: `use-getter-return` · ⚠️ currently shipped in `correctness` |
| `guard-for-in` | ⏳ | eslint |  | biome: `use-guard-for-in` |
| `no-alert` | ⏳ | eslint |  |  |
| `no-approximative-numeric-constant` | ⏳ | eslint |  |  |
| `no-array-index-key` | ⏳ | eslint |  |  |
| `no-assign-in-expressions` | ⏳ | eslint |  |  |
| `no-async-promise-executor` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-bitwise-operators` | ⏳ | biome-original |  |  |
| `no-catch-assign` | ⏳ | eslint |  |  |
| `no-class-assign` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-comment-text` | ⏳ | biome-original |  |  |
| `no-compare-neg-zero` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-confusing-labels` | ⏳ | biome-original |  |  |
| `no-confusing-void-expression` | ✓ | typescript-eslint |  |  |
| `no-confusing-void-type` | ⏳ | typescript-eslint |  |  |
| `no-console` | ⏳ | eslint |  |  |
| `no-const-enum` | ⏳ | eslint |  |  |
| `no-constant-binary-expression` | ✓ | eslint |  | biome: `no-constant-binary-expressions` · ⚠️ currently shipped in `correctness` |
| `no-control-regex` | ✓ | eslint |  | biome: `no-control-characters-in-regex` · ⚠️ currently shipped in `correctness` |
| `no-debugger` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-deprecated` | ✓ | typescript-eslint |  |  |
| `no-deprecated-imports` | ⏳ | biome-original |  |  |
| `no-document-cookie` | ⏳ | biome-original |  |  |
| `no-document-import-in-page` | ⏳ | biome-original |  |  |
| `no-double-equals` | ⏳ | eslint |  |  |
| `no-dupe-args` | ✓ | eslint |  | biome: `no-duplicate-parameters` · ⚠️ currently shipped in `correctness` |
| `no-dupe-class-members` | ✓ | eslint |  | biome: `no-duplicate-class-members` · ⚠️ currently shipped in `correctness` |
| `no-dupe-else-if` | ✓ | eslint |  | biome: `no-duplicate-else-if` · ⚠️ currently shipped in `correctness` |
| `no-dupe-keys` | ✓ | eslint |  | biome: `no-duplicate-object-keys` · ⚠️ currently shipped in `correctness` |
| `no-duplicate-case` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-duplicate-jsx-props` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-duplicate-test-hooks` | ⏳ | biome-original |  |  |
| `no-empty` | ⏳ | eslint |  | biome: `no-empty-block-statements` |
| `no-empty-interface` | ⏳ | eslint |  |  |
| `no-empty-source` | ⏳ | biome-original |  |  |
| `no-evolving-types` | ⏳ | biome-original |  |  |
| `no-explicit-any` | ⏳ | eslint |  |  |
| `no-exports-in-test` | ⏳ | biome-original |  |  |
| `no-extra-non-null-assertion` | ⏳ | eslint |  |  |
| `no-fallthrough` | ✓ | eslint |  | biome: `no-fallthrough-switch-clause` |
| `no-focused-tests` | ⏳ | biome-original |  |  |
| `no-function-assign` | ⏳ | eslint |  |  |
| `no-global-assign` | ⏳ | eslint |  |  |
| `no-global-is-finite` | ⏳ | eslint |  |  |
| `no-global-is-nan` | ⏳ | eslint |  |  |
| `no-head-import-in-document` | ⏳ | biome-original |  |  |
| `no-implicit-any-let` | ⏳ | eslint |  |  |
| `no-import-assign` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-import-cycles` | ⏳ | eslint |  |  |
| `no-instanceof-array` | ⏳ | eslint |  | biome: `use-is-array` |
| `no-irregular-whitespace` | ✓ | eslint |  |  |
| `no-label-var` | ⏳ | eslint |  |  |
| `no-misleading-character-class` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-misplaced-assertion` | ⏳ | biome-original |  |  |
| `no-misrefactored-shorthand-assign` | ⏳ | biome-original |  |  |
| `no-misused-new` | ⏳ | typescript-eslint |  | biome: `no-misleading-instantiator` |
| `no-non-null-asserted-optional-chain` | ⏳ | eslint |  |  |
| `no-octal-escape` | ⏳ | eslint |  |  |
| `no-prototype-builtins` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-react-forward-ref` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-react-specific-props` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-redeclare` | ⏳ | eslint |  |  |
| `no-self-compare` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-shadow-restricted-names` | ⏳ | eslint |  |  |
| `no-skipped-tests` | ⏳ | biome-original |  |  |
| `no-sparse-arrays` | ✓ | eslint |  | biome: `no-sparse-array` · ⚠️ currently shipped in `correctness` |
| `no-suspicious-semicolon-in-jsx` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-template-curly-in-string` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-then-property` | ⏳ | biome-original |  |  |
| `no-ts-ignore` | ⏳ | eslint |  |  |
| `no-unassigned-variables` | ⏳ | eslint |  |  |
| `no-unexpected-multiline` | ✓ | typescript-eslint |  |  |
| `no-unsafe-declaration-merging` | ⏳ | eslint |  |  |
| `no-unsafe-negation` | ✓ | eslint |  | ⚠️ currently shipped in `correctness` |
| `no-unsafe-type-assertion` | ✓ | typescript-eslint |  |  |
| `no-unused-expressions` | ⏳ | eslint |  |  |
| `no-useless-escape-in-string` | ⏳ | eslint |  |  |
| `no-useless-regex-backrefs` | ⏳ | eslint |  |  |
| `no-var` | ⏳ | eslint |  |  |
| `no-with` | ⏳ | eslint |  |  |
| `prefer-namespace-keyword` | ⏳ | eslint |  | biome: `use-namespace-keyword` |
| `promise-function-async` | ✓ | typescript-eslint |  |  |
| `restrict-plus-operands` | ✓ | typescript-eslint |  |  |
| `restrict-template-expressions` | ✓ | typescript-eslint |  |  |
| `return-await` | ✓ | typescript-eslint |  |  |
| `strict` | ⏳ | eslint |  | biome: `no-redundant-use-strict` |
| `strict-boolean-expressions` | ✓ | typescript-eslint |  |  |
| `unbound-method` | ✓ | typescript-eslint |  |  |
| `use-await` | ⏳ | eslint |  |  |
| `use-error-message` | ⏳ | biome-original |  |  |
| `use-google-font-display` | ⏳ | biome-original |  |  |
| `use-iterable-callback-return` | ⏳ | biome-original |  |  |
| `use-number-to-fixed-digits-argument` | ⏳ | biome-original |  |  |
| `use-static-response-methods` | ⏳ | biome-original |  |  |
| `use-strict-mode` | ⏳ | biome-original |  |  |

## security — 6 rules

Patterns enabling injection, eval, prototype pollution, unsafe deserialization, or credential leakage.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `no-blank-target` | ⏳ | biome-original |  |  |
| `no-dangerously-set-inner-html` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-dangerously-set-inner-html-with-children` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-global-eval` | ⏳ | eslint |  |  |
| `no-implied-eval` | ✓ | typescript-eslint |  | biome category: `nursery` — semantic placement applied here |
| `no-secrets` | ⏳ | biome-original |  |  |

## performance — 16 rules

Known-slow patterns with a faster equivalent. No correctness impact.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `no-accumulating-spread` | ⏳ | biome-original |  |  |
| `no-await-in-loop` | ✓ | eslint |  | biome: `no-await-in-loops` |
| `no-barrel-file` | ⏳ | biome-original |  |  |
| `no-delete` | ⏳ | eslint |  |  |
| `no-dynamic-namespace-import-access` | ⏳ | biome-original |  |  |
| `no-img-element` | ⏳ | biome-original |  |  |
| `no-namespace-import` | ⏳ | eslint |  |  |
| `no-re-export-all` | ⏳ | eslint |  |  |
| `no-unwanted-polyfillio` | ⏳ | biome-original |  |  |
| `prefer-find` | ✓ | typescript-eslint |  | biome: `use-find` · biome category: `nursery` — semantic placement applied here |
| `prefer-includes` | ✓ | typescript-eslint |  |  |
| `prefer-regexp-exec` | ✓ | typescript-eslint |  | biome: `use-regexp-exec` · biome category: `nursery` — semantic placement applied here |
| `prefer-string-starts-ends-with` | ✓ | typescript-eslint |  | biome: `use-string-starts-ends-with` · biome category: `nursery` — semantic placement applied here |
| `use-google-font-preconnect` | ⏳ | biome-original |  |  |
| `use-solid-for-component` | ⏳ | eslint-plugin-solid | [solid] |  |
| `use-top-level-regex` | ⏳ | biome-original |  |  |

## complexity — 62 rules

Needless complication with a simpler equivalent. No correctness or perf impact.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `no-adjacent-spaces-in-regex` | ⏳ | eslint |  |  |
| `no-arguments` | ⏳ | eslint |  |  |
| `no-comma-operator` | ⏳ | eslint |  |  |
| `no-duplicate-type-constituents` | ✓ | typescript-eslint |  |  |
| `no-empty-type-parameters` | ⏳ | eslint |  |  |
| `no-excessive-cognitive-complexity` | ⏳ | eslint |  |  |
| `no-excessive-lines-per-function` | ⏳ | eslint |  |  |
| `no-excessive-nested-test-suites` | ⏳ | biome-original |  |  |
| `no-extra-boolean-cast` | ⏳ | eslint |  |  |
| `no-flat-map-identity` | ⏳ | biome-original |  |  |
| `no-for-each` | ⏳ | eslint |  |  |
| `no-implicit-coercion` | ⏳ | biome-original |  | biome: `no-implicit-coercions` |
| `no-redundant-type-constituents` | ✓ | typescript-eslint |  |  |
| `no-restricted-types` | ⏳ | eslint |  | biome: `no-banned-types` |
| `no-static-only-class` | ⏳ | eslint |  |  |
| `no-this-in-static` | ⏳ | eslint |  |  |
| `no-unnecessary-boolean-literal-compare` | ✓ | typescript-eslint |  |  |
| `no-unnecessary-condition` | ✓ | typescript-eslint |  |  |
| `no-unnecessary-qualifier` | ✓ | typescript-eslint |  |  |
| `no-unnecessary-template-expression` | ✓ | typescript-eslint |  | biome category: `nursery` — semantic placement applied here |
| `no-unnecessary-type-arguments` | ✓ | typescript-eslint |  |  |
| `no-unnecessary-type-assertion` | ✓ | typescript-eslint |  |  |
| `no-unnecessary-type-conversion` | ✓ | typescript-eslint |  |  |
| `no-unnecessary-type-parameters` | ✓ | typescript-eslint |  |  |
| `no-useless-catch` | ⏳ | eslint |  |  |
| `no-useless-catch-binding` | ⏳ | eslint |  |  |
| `no-useless-constructor` | ⏳ | eslint |  |  |
| `no-useless-continue` | ⏳ | eslint |  |  |
| `no-useless-default-assignment` | ✓ | typescript-eslint |  |  |
| `no-useless-empty-export` | ⏳ | eslint |  |  |
| `no-useless-escape` | ⏳ | eslint |  | biome: `no-useless-escape-in-regex` |
| `no-useless-fragments` | ⏳ | eslint |  |  |
| `no-useless-label` | ⏳ | eslint |  |  |
| `no-useless-lone-block-statements` | ⏳ | eslint |  |  |
| `no-useless-rename` | ⏳ | eslint |  |  |
| `no-useless-string-concat` | ⏳ | eslint |  |  |
| `no-useless-string-raw` | ⏳ | biome-original |  |  |
| `no-useless-switch-case` | ⏳ | eslint |  |  |
| `no-useless-ternary` | ⏳ | eslint |  |  |
| `no-useless-this-alias` | ⏳ | eslint |  |  |
| `no-useless-type-constraint` | ⏳ | eslint |  |  |
| `no-useless-undefined` | ⏳ | eslint |  |  |
| `no-useless-undefined-initialization` | ⏳ | eslint |  |  |
| `no-void` | ⏳ | eslint |  |  |
| `non-nullable-type-assertion-style` | ✓ | typescript-eslint |  |  |
| `prefer-destructuring` | ✓ | typescript-eslint |  |  |
| `prefer-nullish-coalescing` | ✓ | typescript-eslint |  | biome: `use-nullish-coalescing` · biome category: `nursery` — semantic placement applied here |
| `prefer-optional-chain` | ✓ | typescript-eslint |  |  |
| `prefer-reduce-type-parameter` | ✓ | typescript-eslint |  | biome: `use-reduce-type-parameter` · biome category: `nursery` — semantic placement applied here |
| `prefer-return-this-type` | ✓ | typescript-eslint |  |  |
| `use-arrow-function` | ⏳ | eslint |  |  |
| `use-date-now` | ⏳ | eslint |  |  |
| `use-flat-map` | ⏳ | eslint |  |  |
| `use-index-of` | ⏳ | eslint |  |  |
| `use-literal-keys` | ⏳ | eslint |  |  |
| `use-max-params` | ⏳ | eslint |  |  |
| `use-numeric-literals` | ⏳ | eslint |  |  |
| `use-optional-chain` | ⏳ | eslint |  |  |
| `use-regex-literals` | ⏳ | eslint |  |  |
| `use-simple-number-keys` | ⏳ | eslint |  |  |
| `use-simplified-logic-expression` | ⏳ | eslint |  |  |
| `use-while` | ⏳ | eslint |  |  |

## style — 77 rules

Formatting, naming, ordering. Pure preference; team-configurable.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `consistent-type-exports` | ✓ | typescript-eslint |  | biome: `use-export-type` |
| `consistent-type-imports` | ⏳ | typescript-eslint |  | biome: `use-import-type` |
| `default-case` | ⏳ | eslint |  | biome: `use-default-switch-clause` |
| `dot-notation` | ✓ | typescript-eslint |  |  |
| `naming-convention` | ✓ | typescript-eslint |  |  |
| `no-common-js` | ⏳ | eslint |  |  |
| `no-default-export` | ⏳ | biome-original |  |  |
| `no-done-callback` | ⏳ | eslint |  |  |
| `no-duplicate-imports` | ✓ | typescript-eslint |  |  |
| `no-enum` | ⏳ | eslint |  |  |
| `no-exported-imports` | ⏳ | eslint |  |  |
| `no-head-element` | ⏳ | biome-original |  |  |
| `no-implicit-boolean` | ⏳ | biome-original |  |  |
| `no-inferrable-types` | ⏳ | eslint |  |  |
| `no-jsx-literals` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-magic-numbers` | ⏳ | biome-original |  |  |
| `no-meaningless-void-operator` | ✓ | typescript-eslint |  |  |
| `no-namespace` | ⏳ | eslint |  |  |
| `no-negation-else` | ⏳ | eslint |  |  |
| `no-nested-ternary` | ⏳ | eslint |  |  |
| `no-non-null-assertion` | ⏳ | eslint |  |  |
| `no-parameter-assign` | ⏳ | eslint |  |  |
| `no-parameter-properties` | ⏳ | eslint |  |  |
| `no-process-env` | ⏳ | biome-original |  |  |
| `no-restricted-globals` | ⏳ | eslint |  |  |
| `no-restricted-imports` | ⏳ | eslint |  |  |
| `no-restricted-types` | ⏳ | eslint |  |  |
| `no-shouty-constants` | ⏳ | biome-original |  |  |
| `no-substr` | ⏳ | biome-original |  |  |
| `no-unused-template-literal` | ⏳ | eslint |  |  |
| `no-useless-else` | ⏳ | eslint |  |  |
| `no-yoda-expression` | ⏳ | eslint |  |  |
| `prefer-readonly` | ✓ | typescript-eslint |  |  |
| `prefer-readonly-parameter-types` | ✓ | typescript-eslint |  |  |
| `prefer-template` | ⏳ | eslint |  | biome: `use-template` |
| `use-array-literals` | ⏳ | eslint |  |  |
| `use-as-const-assertion` | ⏳ | eslint |  |  |
| `use-at-index` | ⏳ | eslint |  |  |
| `use-block-statements` | ⏳ | eslint |  |  |
| `use-collapsed-else-if` | ⏳ | eslint |  |  |
| `use-collapsed-if` | ⏳ | eslint |  |  |
| `use-component-export-only-modules` | ⏳ | eslint |  |  |
| `use-consistent-array-type` | ⏳ | eslint |  |  |
| `use-consistent-arrow-return` | ⏳ | eslint |  |  |
| `use-consistent-builtin-instantiation` | ⏳ | eslint |  |  |
| `use-consistent-curly-braces` | ⏳ | eslint |  |  |
| `use-consistent-member-accessibility` | ⏳ | eslint |  |  |
| `use-consistent-object-definitions` | ⏳ | eslint |  |  |
| `use-consistent-type-definitions` | ⏳ | eslint |  |  |
| `use-const` | ⏳ | eslint |  |  |
| `use-default-parameter-last` | ⏳ | eslint |  |  |
| `use-enum-initializers` | ⏳ | eslint |  |  |
| `use-explicit-length-check` | ⏳ | eslint |  |  |
| `use-exponentiation-operator` | ⏳ | eslint |  |  |
| `use-exports-last` | ⏳ | eslint |  |  |
| `use-filenaming-convention` | ⏳ | eslint |  |  |
| `use-for-of` | ⏳ | eslint |  |  |
| `use-fragment-syntax` | ⏳ | eslint |  |  |
| `use-grouped-accessor-pairs` | ⏳ | eslint |  |  |
| `use-literal-enum-members` | ⏳ | eslint |  |  |
| `use-naming-convention` | ⏳ | eslint |  |  |
| `use-node-assert-strict` | ⏳ | eslint |  |  |
| `use-nodejs-import-protocol` | ⏳ | eslint |  |  |
| `use-number-namespace` | ⏳ | eslint |  |  |
| `use-numeric-separators` | ⏳ | eslint |  |  |
| `use-object-spread` | ⏳ | eslint |  |  |
| `use-react-function-components` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `use-readonly-class-properties` | ⏳ | eslint |  |  |
| `use-self-closing-elements` | ⏳ | eslint |  |  |
| `use-shorthand-assign` | ⏳ | eslint |  |  |
| `use-shorthand-function-type` | ⏳ | eslint |  |  |
| `use-single-var-declarator` | ⏳ | eslint |  |  |
| `use-symbol-description` | ⏳ | eslint |  |  |
| `use-throw-new-error` | ⏳ | biome-original |  |  |
| `use-throw-only-error` | ⏳ | biome-original |  |  |
| `use-trim-start-end` | ⏳ | biome-original |  |  |
| `use-unified-type-signatures` | ⏳ | eslint |  |  |

## a11y — 36 rules

JSX accessibility rules. Blocked on JSX-aware analysis support in jetlint.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `no-access-key` | ⏳ | eslint |  |  |
| `no-aria-hidden-on-focusable` | ⏳ | eslint |  |  |
| `no-aria-unsupported-elements` | ⏳ | eslint |  |  |
| `no-autofocus` | ⏳ | eslint |  |  |
| `no-distracting-elements` | ⏳ | eslint |  |  |
| `no-header-scope` | ⏳ | eslint |  |  |
| `no-interactive-element-to-noninteractive-role` | ⏳ | eslint |  |  |
| `no-label-without-control` | ⏳ | eslint |  |  |
| `no-noninteractive-element-interactions` | ⏳ | eslint |  |  |
| `no-noninteractive-element-to-interactive-role` | ⏳ | eslint |  |  |
| `no-noninteractive-tabindex` | ⏳ | eslint |  |  |
| `no-positive-tabindex` | ⏳ | eslint |  |  |
| `no-redundant-alt` | ⏳ | eslint |  |  |
| `no-redundant-roles` | ⏳ | eslint |  |  |
| `no-static-element-interactions` | ⏳ | eslint |  |  |
| `no-svg-without-title` | ⏳ | eslint |  |  |
| `use-alt-text` | ⏳ | eslint |  |  |
| `use-anchor-content` | ⏳ | eslint |  |  |
| `use-aria-activedescendant-with-tabindex` | ⏳ | eslint |  |  |
| `use-aria-props-for-role` | ⏳ | eslint |  |  |
| `use-aria-props-supported-by-role` | ⏳ | eslint |  |  |
| `use-button-type` | ⏳ | eslint |  |  |
| `use-focusable-interactive` | ⏳ | eslint |  |  |
| `use-heading-content` | ⏳ | eslint |  |  |
| `use-html-lang` | ⏳ | eslint |  |  |
| `use-iframe-title` | ⏳ | eslint |  |  |
| `use-key-with-click-events` | ⏳ | eslint |  |  |
| `use-key-with-mouse-events` | ⏳ | eslint |  |  |
| `use-media-caption` | ⏳ | eslint |  |  |
| `use-semantic-elements` | ⏳ | eslint |  |  |
| `use-valid-anchor` | ⏳ | eslint |  |  |
| `use-valid-aria-props` | ⏳ | eslint |  |  |
| `use-valid-aria-role` | ⏳ | eslint |  |  |
| `use-valid-aria-values` | ⏳ | eslint |  |  |
| `use-valid-autocomplete` | ⏳ | eslint |  |  |
| `use-valid-lang` | ⏳ | eslint |  |  |

## nursery — 95 rules

Rules whose **behavior** is still iterating (jetlint's `Stability: nursery`). Distinct from biome's `nursery`, which is an organizational state.

| Rule | Status | Source | Tags | Notes |
|---|:-:|---|---|---|
| `no-ambiguous-anchor-text` | ⏳ | eslint |  |  |
| `no-before-interactive-script-outside-document` | ⏳ | eslint |  |  |
| `no-component-hook-factories` | ⏳ | eslint |  |  |
| `no-conditional-expect` | ⏳ | eslint |  |  |
| `no-continue` | ⏳ | eslint |  |  |
| `no-div-regex` | ⏳ | eslint |  |  |
| `no-drizzle-delete-without-where` | ⏳ | eslint-plugin-drizzle | [drizzle] |  |
| `no-drizzle-update-without-where` | ⏳ | eslint-plugin-drizzle | [drizzle] |  |
| `no-duplicate-enum-values` | ⏳ | eslint |  |  |
| `no-duplicated-spread-props` | ⏳ | eslint |  |  |
| `no-equals-to-null` | ⏳ | eslint |  |  |
| `no-excessive-classes-per-file` | ⏳ | eslint |  |  |
| `no-excessive-lines-per-file` | ⏳ | eslint |  |  |
| `no-excessive-nested-callbacks` | ⏳ | eslint |  |  |
| `no-floating-classes` | ⏳ | typescript-eslint |  |  |
| `no-for-in` | ⏳ | eslint |  |  |
| `no-identical-test-title` | ⏳ | eslint |  |  |
| `no-increment-decrement` | ⏳ | eslint |  |  |
| `no-inline-styles` | ⏳ | eslint |  |  |
| `no-jsx-leaked-dollar` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-jsx-namespace` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-jsx-props-bind` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-leaked-render` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-loop-func` | ⏳ | eslint |  |  |
| `no-misleading-return-type` | ⏳ | eslint |  |  |
| `no-multi-assign` | ⏳ | eslint |  |  |
| `no-multi-str` | ⏳ | eslint |  |  |
| `no-nested-promises` | ⏳ | eslint |  |  |
| `no-parameters-only-used-in-recursion` | ⏳ | eslint |  |  |
| `no-playwright-element-handle` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-eval` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-force-option` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-missing-await` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-networkidle` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-page-pause` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-useless-await` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-wait-for-navigation` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-wait-for-selector` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-playwright-wait-for-timeout` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `no-proto` | ⏳ | eslint |  |  |
| `no-react-native-deep-imports` | ⏳ | eslint-plugin-react-native | [react-native] |  |
| `no-react-native-literal-colors` | ⏳ | eslint-plugin-react-native | [react-native] |  |
| `no-react-native-raw-text` | ⏳ | eslint-plugin-react-native | [react-native] |  |
| `no-react-string-refs` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `no-redundant-default-export` | ⏳ | eslint |  |  |
| `no-return-assign` | ⏳ | eslint |  |  |
| `no-script-url` | ⏳ | eslint |  |  |
| `no-shadow` | ⏳ | eslint |  |  |
| `no-sync-scripts` | ⏳ | eslint |  |  |
| `no-ternary` | ⏳ | eslint |  |  |
| `no-undeclared-env-vars` | ⏳ | eslint |  |  |
| `no-unknown-attribute` | ⏳ | eslint |  |  |
| `no-unnecessary-conditions` | ⏳ | typescript-eslint |  |  |
| `no-unsafe-plus-operands` | ⏳ | eslint |  |  |
| `no-useless-return` | ⏳ | eslint |  |  |
| `no-useless-type-conversion` | ⏳ | eslint |  |  |
| `no-vue-arrow-func-in-watch` | ⏳ | eslint-plugin-vue | [vue] |  |
| `no-vue-import-compiler-macros` | ⏳ | eslint-plugin-vue | [vue] |  |
| `no-vue-options-api` | ⏳ | eslint-plugin-vue | [vue] |  |
| `no-vue-ref-as-operand` | ⏳ | eslint-plugin-vue | [vue] |  |
| `prefer-array-some` | ⏳ | eslint |  | biome: `use-array-some` |
| `prefer-spread` | ⏳ | eslint |  | biome: `use-spread` |
| `use-consistent-enum-value-type` | ⏳ | eslint |  |  |
| `use-consistent-method-signatures` | ⏳ | eslint |  |  |
| `use-consistent-test-it` | ⏳ | eslint |  |  |
| `use-destructuring` | ⏳ | eslint |  |  |
| `use-disposables` | ⏳ | eslint |  |  |
| `use-dom-node-text-content` | ⏳ | eslint |  |  |
| `use-dom-query-selector` | ⏳ | eslint |  |  |
| `use-error-cause` | ⏳ | eslint |  |  |
| `use-exhaustive-switch-cases` | ⏳ | eslint |  |  |
| `use-expect` | ⏳ | eslint |  |  |
| `use-explicit-return-type` | ⏳ | eslint |  |  |
| `use-explicit-type` | ⏳ | eslint |  |  |
| `use-global-this` | ⏳ | eslint |  |  |
| `use-iframe-sandbox` | ⏳ | eslint |  |  |
| `use-imports-first` | ⏳ | eslint |  |  |
| `use-inline-script-id` | ⏳ | eslint |  |  |
| `use-math-min-max` | ⏳ | eslint |  |  |
| `use-named-capture-group` | ⏳ | eslint |  |  |
| `use-playwright-valid-describe-callback` | ⏳ | eslint-plugin-playwright | [playwright] |  |
| `use-qwik-loader-location` | ⏳ | eslint-plugin-qwik | [qwik] |  |
| `use-react-async-server-function` | ⏳ | eslint-plugin-react / -react-hooks / jsx-a11y | [react] |  |
| `use-react-native-platform-components` | ⏳ | eslint-plugin-react-native | [react-native] |  |
| `use-regexp-test` | ⏳ | eslint |  |  |
| `use-sorted-classes` | ⏳ | eslint |  |  |
| `use-test-hooks-in-order` | ⏳ | eslint |  |  |
| `use-test-hooks-on-top` | ⏳ | eslint |  |  |
| `use-this-in-class-methods` | ⏳ | eslint |  |  |
| `use-unicode-regex` | ⏳ | eslint |  |  |
| `use-vars-on-top` | ⏳ | eslint |  |  |
| `use-vue-consistent-define-props-declaration` | ⏳ | eslint-plugin-vue | [vue] |  |
| `use-vue-define-macros-order` | ⏳ | eslint-plugin-vue | [vue] |  |
| `use-vue-multi-word-component-names` | ⏳ | eslint-plugin-vue | [vue] |  |
| `use-vue-next-tick-promise` | ⏳ | eslint-plugin-vue | [vue] |  |

## Per-category notes on contested placements

Rules flagged ⚠️ above currently ship in a different jetlint category than this catalog proposes. We follow biome's lead in each case; reverting any specific decision is a one-field registry edit.

- **`no-self-compare`** (→ `suspicious`, from `correctness`): the pattern is sometimes a legitimate NaN check.
- **`no-prototype-builtins`** (→ `suspicious`, from `correctness`): legitimate when the object provably isn't a plain `Object.prototype` consumer.
- **`no-import-assign`** (→ `suspicious`, from `correctness`): the assignment is detectable but doesn't always indicate a bug in TS source.
- **`getter-return`** (→ `suspicious`, from `correctness`): a getter that intentionally throws or has side-effects is a valid pattern.
- **`no-control-regex`** (→ `suspicious`, from `correctness`): a deliberate control char is rare but plausible (e.g. binary protocols).
- **`no-constant-condition`** (→ `correctness`, from `suspicious`): biome's framing wins. Counter-example `while (true) {}` is intentional, but biome treats it as correctness.

## Open question (catalog placeholder, no decision yet)

- **`use-iterable-callback-return` vs `array-callback-return`**: these are different shapes. eslint's `array-callback-return` (which jetlint ships) covers Array methods only; biome's `use-iterable-callback-return` covers all iterables (generators, Sets, Maps, custom protocols). Following biome's lead eventually means either renaming + expanding the existing rule, or shipping the broader version as a separate rule and deprecating the narrow one. Behavior change either way; defer until we decide whether to broaden semantics.

## Cross-cutting work not tracked in the catalog

These don't map to a category Milestone and live as their own issues:

- **JSX/TSX parser support**. Blocker for every a11y rule and every framework-tagged rule.
- **Framework preset configurations**. Once JSX lands, group framework rules behind preset names (`jetlint:react`, `jetlint:vue`) rather than asking users to enable each rule by ID.
- **Biome-original rules without upstream fixtures** (e.g. `no-secrets`, `no-barrel-file`). Require hand-built test corpora since there's no oxc port to vendor from.
- **Aliases and renames**. Biome's names diverge from eslint/ts-eslint in many places (see Notes columns). A future config option may let users write either name.

## Frameworks are tracked as labels, not Milestones

Framework-specific rules (React, Vue, Solid, Qwik, Next, Playwright, Drizzle, React Native) get a GitHub issue label of the form `framework:<name>`, not their own Milestone. The rationale:

- GitHub issues belong to exactly one Milestone. A React `correctness` rule would have to pick between "correctness" and "react" — and either choice loses one view.
- Labels are multi-assignable. An issue can have both `framework:react` and the `correctness` Milestone, and the GitHub Issues UI can filter on both simultaneously.
- Most framework rules can't ship until JSX-aware analysis lands; eight per-framework Milestones would be eight progress bars stuck at 0% for the foreseeable future.

To find every React rule across all categories: `is:issue label:framework:react`. To find correctness rules that are not framework-specific: `is:issue milestone:correctness -label:framework:*`. The Milestones index page (`/milestones`) shows aggregate progress per category.

