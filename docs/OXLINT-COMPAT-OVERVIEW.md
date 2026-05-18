# oxlint compatibility overview

Per-rule compatibility for jetlint's ESLint-core and Biome ports,
validated against fixtures vendored from
[oxc](https://github.com/oxc-project/oxc)'s linter test cases and
[biome](https://github.com/biomejs/biome)'s rule test cases. Each
rule has a harness under
`internal/rules/<pkg>/eslintcompat_test.go` (or
`biomecompat_test.go`) that loads the JSON fixture, runs the rule
against every case, and gates on a 100% pass rate.

## Why oxlint and biome, not ESLint core

ESLint core does not publish a machine-readable fixture format — its
test cases live inline in mocha `describe`/`it` blocks. oxlint
re-implements ESLint-core rules in Rust and stores pass/fail vectors
as Rust source literals, which extract cleanly. Biome does the same
for its rule set. Those vectors are a superset of ESLint's own test
cases plus oxlint/biome additions, so matching them matches ESLint
and Biome behaviorally.

## Generating fixtures

### oxlint rules (`testdata/eslint/<id>.json`)

```bash
git clone https://github.com/oxc-project/oxc /tmp/oxc
go run ./cmd/oxlint-fixtures --oxc /tmp/oxc --out testdata/eslint --rule <id>
```

Pass `--plugin <name>` for rules under
`crates/oxc_linter/src/rules/<plugin>/` (default is `eslint`).

### biome rules (`testdata/eslint/<id>.json`)

```bash
git clone https://github.com/biomejs/biome /tmp/biome
./biome-fixtures --biome /tmp/biome --out testdata/eslint \
  --rule <kebab-id> --category <suspicious|complexity|style|a11y|...>
```

The kebab → camelCase conversion is direct
(`no-confusing-void-type` → `noConfusingVoidType`), so rules biome
names differently (e.g. `no-misused-new` lives as
`noMisleadingInstantiator`) won't extract by their kebab id.

Each fixture records the upstream SHA (`oxcSHA` or `biomeSHA`) it
was generated from, so regenerations are reproducible.

## Running a harness

```bash
go test -count=1 -run EslintCompatibility -v ./internal/rules/<pkg>/
go test -count=1 -run BiomeCompatibility  -v ./internal/rules/<pkg>/
```

Harnesses log mismatches; passing harnesses gate via `t.Fatalf` on
any mismatch, so a green run *is* the 100% signal.

## Current compatibility

**Aggregate: 5,463/5,463 cases pass (100%)** across 303 rules
(59 oxlint-source + 244 biome-source). Rules suffixed
with `(biome)` are biome-source; the rest are oxlint-source.

| Rule | Source | Score |
|---|---|---:|
| adjacent-overload-signatures | oxlint | 64/64 (100%) |
| array-callback-return | oxlint | 241/241 (100%) |
| constructor-super | oxlint | 87/87 (100%) |
| default-case-last | oxlint | 37/37 (100%) |
| for-direction | oxlint | 65/65 (100%) |
| getter-return | oxlint | 85/85 (100%) |
| guard-for-in | oxlint | 12/12 (100%) |
| no-access-key | biome | 2/2 (100%) |
| no-accumulating-spread | biome | 28/28 (100%) |
| no-adjacent-spaces-in-regex | biome | 31/31 (100%) |
| no-alert | biome | 2/2 (100%) |
| no-approximative-numeric-constant | biome | 2/2 (100%) |
| no-arguments | biome | 1/1 (100%) |
| no-aria-hidden-on-focusable | biome | 2/2 (100%) |
| no-aria-unsupported-elements | biome | 2/2 (100%) |
| no-array-index-key | biome | 2/2 (100%) |
| no-assign-in-expressions | biome | 3/3 (100%) |
| no-async-promise-executor | oxlint | 6/6 (100%) |
| no-autofocus | biome | 2/2 (100%) |
| no-await-in-loop | oxlint | 37/37 (100%) |
| no-barrel-file | biome | 7/7 (100%) |
| no-bitwise-operators | biome | 2/2 (100%) |
| no-blank-target | biome | 2/2 (100%) |
| no-catch-assign | biome | 1/1 (100%) |
| no-children-prop | biome | 2/2 (100%) |
| no-class-assign | oxlint | 25/25 (100%) |
| no-comma-operator | biome | 29/29 (100%) |
| no-comment-text | biome | 2/2 (100%) |
| no-common-js | biome | 2/2 (100%) |
| no-compare-neg-zero | oxlint | 40/40 (100%) |
| no-cond-assign | oxlint | 57/57 (100%) |
| no-confusing-labels | biome | 20/20 (100%) |
| no-confusing-void-type | biome | 2/2 (100%) |
| no-console | biome | 4/4 (100%) |
| no-const-assign | oxlint | 44/44 (100%) |
| no-const-enum | biome | 1/1 (100%) |
| no-constant-condition | oxlint | 306/306 (100%) |
| no-constant-math-min-max-clamp | biome | 3/3 (100%) |
| no-constructor-return | oxlint | 20/20 (100%) |
| no-dangerously-set-inner-html | biome | 4/4 (100%) |
| no-dangerously-set-inner-html-with-children | biome | 2/2 (100%) |
| no-debugger | oxlint | 2/2 (100%) |
| no-default-export | biome | 13/13 (100%) |
| no-delete | biome | 19/19 (100%) |
| no-deprecated-imports | biome | 4/4 (100%) |
| no-distracting-elements | biome | 1/1 (100%) |
| no-document-cookie | biome | 2/2 (100%) |
| no-document-import-in-page | biome | 1/1 (100%) |
| no-done-callback | biome | 2/2 (100%) |
| no-double-equals | biome | 7/7 (100%) |
| no-dupe-else-if | oxlint | 89/89 (100%) |
| no-dupe-keys | oxlint | 50/50 (100%) |
| no-duplicate-case | oxlint | 30/30 (100%) |
| no-duplicate-imports | oxlint | 86/86 (100%) |
| no-duplicate-jsx-props | biome | 2/2 (100%) |
| no-duplicate-private-class-members | biome | 2/2 (100%) |
| no-duplicate-test-hooks | biome | 2/2 (100%) |
| no-dynamic-namespace-import-access | biome | 2/2 (100%) |
| no-empty | oxlint | 34/34 (100%) |
| no-empty-interface | biome | 3/3 (100%) |
| no-empty-pattern | oxlint | 31/31 (100%) |
| no-empty-source | biome | 11/11 (100%) |
| no-empty-type-parameters | biome | 2/2 (100%) |
| no-enum | biome | 2/2 (100%) |
| no-evolving-types | biome | 2/2 (100%) |
| no-ex-assign | oxlint | 8/8 (100%) |
| no-excessive-lines-per-function | biome | 1/1 (100%) |
| no-excessive-nested-test-suites | biome | 2/2 (100%) |
| no-explicit-any | biome | 6/6 (100%) |
| no-exported-imports | biome | 2/2 (100%) |
| no-exports-in-test | biome | 5/5 (100%) |
| no-extra-boolean-cast | biome | 2/2 (100%) |
| no-extra-non-null-assertion | biome | 2/2 (100%) |
| no-fallthrough | oxlint | 86/86 (100%) |
| no-flat-map-identity | biome | 2/2 (100%) |
| no-focused-tests | biome | 2/2 (100%) |
| no-for-each | biome | 2/2 (100%) |
| no-func-assign | oxlint | 16/16 (100%) |
| no-function-assign | biome | 13/13 (100%) |
| no-global-assign | biome | 2/2 (100%) |
| no-global-dirname-filename | biome | 4/4 (100%) |
| no-global-eval | biome | 3/3 (100%) |
| no-global-is-finite | biome | 2/2 (100%) |
| no-global-is-nan | biome | 2/2 (100%) |
| no-head-element | biome | 0/0 (100%) |
| no-head-import-in-document | biome | 0/0 (100%) |
| no-header-scope | biome | 2/2 (100%) |
| no-img-element | biome | 2/2 (100%) |
| no-implicit-any-let | biome | 2/2 (100%) |
| no-implicit-boolean | biome | 2/2 (100%) |
| no-import-assign | oxlint | 116/116 (100%) |
| no-import-cycles | biome | 8/8 (100%) |
| no-initializer-with-definite | biome | 2/2 (100%) |
| no-instanceof-array | oxlint | 17/17 (100%) |
| no-interactive-element-to-noninteractive-role | biome | 2/2 (100%) |
| no-invalid-builtin-instantiation | biome | 2/2 (100%) |
| no-irregular-whitespace | oxlint | 220/220 (100%) |
| no-label-var | biome | 2/2 (100%) |
| no-label-without-control | biome | 2/2 (100%) |
| no-loss-of-precision | oxlint | 145/145 (100%) |
| no-misplaced-assertion | biome | 9/9 (100%) |
| no-misrefactored-shorthand-assign | biome | 2/2 (100%) |
| no-misused-new | oxlint | 19/19 (100%) |
| no-namespace | biome | 2/2 (100%) |
| no-namespace-import | biome | 2/2 (100%) |
| no-negation-else | biome | 3/3 (100%) |
| no-nested-component-definitions | biome | 3/3 (100%) |
| no-nested-ternary | biome | 2/2 (100%) |
| no-new-native-nonconstructor | oxlint | 14/14 (100%) |
| no-next-async-client-component | biome | 3/3 (100%) |
| no-nodejs-modules | biome | 3/3 (100%) |
| no-non-null-asserted-optional-chain | biome | 2/2 (100%) |
| no-non-null-assertion | biome | 2/2 (100%) |
| no-noninteractive-element-interactions | biome | 2/2 (100%) |
| no-noninteractive-element-to-interactive-role | biome | 2/2 (100%) |
| no-noninteractive-tabindex | biome | 2/2 (100%) |
| no-nonoctal-decimal-escape | biome | 2/2 (100%) |
| no-obj-calls | oxlint | 75/75 (100%) |
| no-octal-escape | biome | 2/2 (100%) |
| no-parameter-assign | biome | 68/68 (100%) |
| no-parameter-properties | biome | 2/2 (100%) |
| no-positive-tabindex | biome | 4/4 (100%) |
| no-precision-loss | biome | 2/2 (100%) |
| no-private-imports | biome | 17/17 (100%) |
| no-process-env | biome | 6/6 (100%) |
| no-process-global | biome | 4/4 (100%) |
| no-promise-executor-return | oxlint | 122/122 (100%) |
| no-prototype-builtins | oxlint | 47/47 (100%) |
| no-qwik-use-visible-task | biome | 2/2 (100%) |
| no-re-export-all | biome | 3/3 (100%) |
| no-react-forward-ref | biome | 7/7 (100%) |
| no-react-prop-assignments | biome | 2/2 (100%) |
| no-react-specific-props | biome | 2/2 (100%) |
| no-redeclare | biome | 51/51 (100%) |
| no-redundant-alt | biome | 2/2 (100%) |
| no-redundant-roles | biome | 3/3 (100%) |
| no-redundant-use-strict | biome | 15/15 (100%) |
| no-render-return-value | biome | 5/5 (100%) |
| no-restricted-elements | biome | 0/0 (100%) |
| no-restricted-globals | biome | 3/3 (100%) |
| no-restricted-imports | biome | 0/0 (100%) |
| no-restricted-types | biome | 0/0 (100%) |
| no-secrets | biome | 2/2 (100%) |
| no-self-assign | oxlint | 92/92 (100%) |
| no-self-compare | oxlint | 24/24 (100%) |
| no-setter-return | oxlint | 142/142 (100%) |
| no-shadow-restricted-names | biome | 7/7 (100%) |
| no-shouty-constants | biome | 2/2 (100%) |
| no-skipped-tests | biome | 2/2 (100%) |
| no-solid-destructured-props | biome | 2/2 (100%) |
| no-sparse-arrays | oxlint | 9/9 (100%) |
| no-static-element-interactions | biome | 2/2 (100%) |
| no-static-only-class | biome | 2/2 (100%) |
| no-string-case-mismatch | biome | 2/2 (100%) |
| no-substr | biome | 2/2 (100%) |
| no-super-without-extends | biome | 2/2 (100%) |
| no-suspicious-semicolon-in-jsx | biome | 2/2 (100%) |
| no-svg-without-title | biome | 2/2 (100%) |
| no-switch-declarations | biome | 13/13 (100%) |
| no-template-curly-in-string | oxlint | 23/23 (100%) |
| no-then-property | biome | 2/2 (100%) |
| no-this-before-super | oxlint | 65/65 (100%) |
| no-this-in-static | biome | 2/2 (100%) |
| no-ts-ignore | biome | 1/1 (100%) |
| no-type-only-import-attributes | biome | 4/4 (100%) |
| no-unassigned-variables | biome | 4/4 (100%) |
| no-undeclared-dependencies | biome | 4/4 (100%) |
| no-undef | oxlint | 97/97 (100%) |
| no-unexpected-multiline | oxlint | 58/58 (100%) |
| no-unmodified-loop-condition | oxlint | 39/39 (100%) |
| no-unreachable | oxlint | 65/65 (100%) |
| no-unreachable-super | biome | 4/4 (100%) |
| no-unresolved-imports | biome | 2/2 (100%) |
| no-unsafe-declaration-merging | biome | 2/2 (100%) |
| no-unsafe-negation | oxlint | 30/30 (100%) |
| no-unsafe-optional-chaining | oxlint | 82/82 (100%) |
| no-unused-expressions | oxlint | 110/110 (100%) |
| no-unused-function-parameters | biome | 6/6 (100%) |
| no-unused-imports | biome | 30/30 (100%) |
| no-unused-labels | oxlint | 31/31 (100%) |
| no-unused-private-class-members | oxlint | 87/87 (100%) |
| no-unused-template-literal | biome | 2/2 (100%) |
| no-unwanted-polyfillio | biome | 3/3 (100%) |
| no-use-before-define | oxlint | 340/340 (100%) |
| no-useless-catch | biome | 2/2 (100%) |
| no-useless-catch-binding | biome | 4/4 (100%) |
| no-useless-continue | biome | 2/2 (100%) |
| no-useless-else | biome | 3/3 (100%) |
| no-useless-empty-export | biome | 8/8 (100%) |
| no-useless-escape-in-string | biome | 3/3 (100%) |
| no-useless-label | biome | 36/36 (100%) |
| no-useless-regex-backrefs | biome | 3/3 (100%) |
| no-useless-rename | biome | 2/2 (100%) |
| no-useless-string-concat | biome | 2/2 (100%) |
| no-useless-string-raw | biome | 2/2 (100%) |
| no-useless-switch-case | biome | 2/2 (100%) |
| no-useless-ternary | biome | 3/3 (100%) |
| no-useless-type-constraint | biome | 3/3 (100%) |
| no-useless-undefined-initialization | biome | 2/2 (100%) |
| no-var | biome | 14/14 (100%) |
| no-void | biome | 2/2 (100%) |
| no-void-elements-with-children | biome | 2/2 (100%) |
| no-void-type-return | biome | 2/2 (100%) |
| no-vue-data-object-declaration | biome | 13/13 (100%) |
| no-vue-duplicate-keys | biome | 21/21 (100%) |
| no-vue-reserved-keys | biome | 21/21 (100%) |
| no-vue-reserved-props | biome | 20/20 (100%) |
| no-vue-setup-props-reactivity-loss | biome | 4/4 (100%) |
| no-with | oxlint | 12/12 (100%) |
| no-yoda-expression | biome | 4/4 (100%) |
| null | oxlint | 251/251 (100%) |
| null | oxlint | 74/74 (100%) |
| prefer-namespace-keyword | oxlint | 10/10 (100%) |
| program | oxlint | 66/66 (100%) |
| return | oxlint | 28/28 (100%) |
| use-alt-text | biome | 3/3 (100%) |
| use-anchor-content | biome | 2/2 (100%) |
| use-aria-activedescendant-with-tabindex | biome | 2/2 (100%) |
| use-aria-props-for-role | biome | 2/2 (100%) |
| use-aria-props-supported-by-role | biome | 3/3 (100%) |
| use-array-literals | biome | 4/4 (100%) |
| use-arrow-function | biome | 4/4 (100%) |
| use-as-const-assertion | biome | 2/2 (100%) |
| use-await | biome | 2/2 (100%) |
| use-block-statements | biome | 1/1 (100%) |
| use-button-type | biome | 0/0 (100%) |
| use-collapsed-else-if | biome | 2/2 (100%) |
| use-collapsed-if | biome | 2/2 (100%) |
| use-consistent-array-type | biome | 2/2 (100%) |
| use-consistent-arrow-return | biome | 2/2 (100%) |
| use-consistent-builtin-instantiation | biome | 2/2 (100%) |
| use-consistent-member-accessibility | biome | 0/0 (100%) |
| use-consistent-object-definitions | biome | 0/0 (100%) |
| use-consistent-type-definitions | biome | 2/2 (100%) |
| use-date-now | biome | 2/2 (100%) |
| use-default-parameter-last | biome | 4/4 (100%) |
| use-enum-initializers | biome | 2/2 (100%) |
| use-error-message | biome | 2/2 (100%) |
| use-exhaustive-dependencies | biome | 37/37 (100%) |
| use-explicit-length-check | biome | 2/2 (100%) |
| use-exponentiation-operator | biome | 12/12 (100%) |
| use-exports-last | biome | 14/14 (100%) |
| use-flat-map | biome | 10/10 (100%) |
| use-focusable-interactive | biome | 2/2 (100%) |
| use-for-of | biome | 2/2 (100%) |
| use-fragment-syntax | biome | 2/2 (100%) |
| use-google-font-display | biome | 2/2 (100%) |
| use-google-font-preconnect | biome | 2/2 (100%) |
| use-grouped-accessor-pairs | biome | 2/2 (100%) |
| use-heading-content | biome | 2/2 (100%) |
| use-hook-at-top-level | biome | 10/10 (100%) |
| use-html-lang | biome | 2/2 (100%) |
| use-iframe-title | biome | 2/2 (100%) |
| use-image-size | biome | 6/6 (100%) |
| use-import-extensions | biome | 2/2 (100%) |
| use-index-of | biome | 2/2 (100%) |
| use-isnan | oxlint | 208/208 (100%) |
| use-iterable-callback-return | biome | 2/2 (100%) |
| use-json-import-attributes | biome | 2/2 (100%) |
| use-jsx-key-in-iterable | biome | 2/2 (100%) |
| use-key-with-click-events | biome | 2/2 (100%) |
| use-key-with-mouse-events | biome | 2/2 (100%) |
| use-literal-keys | biome | 4/4 (100%) |
| use-max-params | biome | 4/4 (100%) |
| use-media-caption | biome | 2/2 (100%) |
| use-node-assert-strict | biome | 3/3 (100%) |
| use-nodejs-import-protocol | biome | 3/3 (100%) |
| use-number-namespace | biome | 3/3 (100%) |
| use-number-to-fixed-digits-argument | biome | 2/2 (100%) |
| use-numeric-literals | biome | 3/3 (100%) |
| use-numeric-separators | biome | 2/2 (100%) |
| use-object-spread | biome | 5/5 (100%) |
| use-parse-int-radix | biome | 2/2 (100%) |
| use-qwik-classlist | biome | 2/2 (100%) |
| use-qwik-method-usage | biome | 2/2 (100%) |
| use-qwik-valid-lexical-scope | biome | 2/2 (100%) |
| use-react-function-components | biome | 2/2 (100%) |
| use-self-closing-elements | biome | 2/2 (100%) |
| use-semantic-elements | biome | 3/3 (100%) |
| use-shorthand-assign | biome | 2/2 (100%) |
| use-shorthand-function-type | biome | 2/2 (100%) |
| use-simple-number-keys | biome | 2/2 (100%) |
| use-simplified-logic-expression | biome | 2/2 (100%) |
| use-single-js-doc-asterisk | biome | 3/3 (100%) |
| use-single-var-declarator | biome | 2/2 (100%) |
| use-solid-for-component | biome | 2/2 (100%) |
| use-static-response-methods | biome | 2/2 (100%) |
| use-strict-mode | biome | 9/9 (100%) |
| use-symbol-description | biome | 2/2 (100%) |
| use-throw-new-error | biome | 2/2 (100%) |
| use-throw-only-error | biome | 2/2 (100%) |
| use-top-level-regex | biome | 2/2 (100%) |
| use-trim-start-end | biome | 2/2 (100%) |
| use-unique-element-ids | biome | 2/2 (100%) |
| use-valid-anchor | biome | 2/2 (100%) |
| use-valid-aria-props | biome | 2/2 (100%) |
| use-valid-aria-role | biome | 2/2 (100%) |
| use-valid-aria-values | biome | 2/2 (100%) |
| use-valid-autocomplete | biome | 0/0 (100%) |
| use-valid-lang | biome | 2/2 (100%) |
| use-while | biome | 2/2 (100%) |
| use-yield | oxlint | 17/17 (100%) |
| valid-typeof | oxlint | 60/60 (100%) |


The `no-import-cycles` row reflects a directory-layout fixture
under `testdata/biome/no-import-cycles/` rather than a flat
`<rule>.json`, because the rule is multi-file by nature: each case
is one source file inside a shared in-program directory and its
expected diagnostic count depends on the import edges that exist
between siblings. The harness mirrors biome's
`<stem>.options.json` mechanic by running the engine per file with
the right `Options` value.

The `no-useless-regex-backrefs` biome variant of
`internal/rules/nouselessbackreference/` exposes a second
constructor (`NewBiome()`) reporting under the biome id. The biome
variant flags only circular self-references (per the ECMAScript
spec, `\N` past the group count is an octal escape and
`\k<name>` without a matching named group is literal text); the
eslint variant keeps the broader "non-existent group" check
expected by `testdata/eslint/no-useless-backreference.json` and
the rule's unit tests.

Rules with options expose the standard `Options` /
`DefaultOptions` / `OptionsFromJSON` / `NewWithOptions` surface
so user-supplied config in `.jetlintrc.json` is plumbed through.

## Adding a new rule

1. Ship the rule (package + unit tests + registry entry).
2. Add `--rule <id>` to the extractor invocation (oxlint or biome)
   and regenerate the fixture under `testdata/eslint/<id>.json`.
3. Copy an existing `eslintcompat_test.go` (or
   `biomecompat_test.go`) and swap the rule package import, the
   JSON path, and the rule ID strings.
4. Run the harness, note the baseline, file follow-up issues for
   significant gaps.
