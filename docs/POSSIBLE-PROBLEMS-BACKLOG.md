# ESLint "Possible Problems" porting backlog

The complete list of ESLint-core "Possible Problems" rules, in
alphabetical order. Working through these in order, one rule per PR.
Update this file as rules land or assumptions change.

**Status counts**

- Shipped: 57 (all rules have implementations; 8 are partial-coverage and need work to reach 100%)
- Remaining (AST-only): 0
- Remaining (needs regex AST infra): 0
- Partials needing 100%: no-constant-condition (206/306), no-constant-binary-expression (176/251), no-duplicate-imports (76/86), no-inner-declarations (65/66), no-undef (87/97), no-unmodified-loop-condition (37/39), no-use-before-define (239/354)

**Symbols**

- ✓ shipped
- … in progress
- ⊘ needs new infra (regex AST) — deferred until the infra lands

## Backlog

| Rule | Status | Notes |
|---|:-:|---|
| `array-callback-return` | ✓ | Shipped 2026-05-15 |
| `constructor-super` | ✓ | Shipped 2026-05-15 |
| `for-direction` | ✓ | Shipped 2026-05-15 |
| `getter-return` | ✓ | Shipped 2026-05-15 |
| `no-async-promise-executor` | ✓ | Shipped 2026-05-15 |
| `no-await-in-loop` | ✓ | Shipped 2026-05-15 |
| `no-class-assign` | ✓ | Shipped 2026-05-15 |
| `no-compare-neg-zero` | ✓ | Shipped 2026-05-15 |
| `no-cond-assign` | ✓ | Shipped 2026-05-15 |
| `no-const-assign` | ✓ | Shipped 2026-05-15 |
| `no-constant-binary-expression` | ✓ | Shipped 2026-05-15 (conservative port, 176/251 fixtures; full constant folding deferred to a shared utility) |
| `no-constant-condition` | ✓ | Shipped 2026-05-15 (conservative port, 206/306 fixtures; full constant-folding deferred to a shared utility) |
| `no-constructor-return` | ✓ | Shipped 2026-05-15 |
| `no-control-regex` | ✓ | Shipped 2026-05-15 (hand-rolled pattern scanner with JS string-escape unescaping; covers `/.../` and `RegExp(string)` forms) |
| `no-debugger` | ✓ | Shipped 2026-05-15 |
| `no-dupe-args` | ✓ | Shipped 2026-05-15 (hand-written tests; no oxc source available) |
| `no-dupe-class-members` | ✓ | Shipped 2026-05-15 |
| `no-dupe-else-if` | ✓ | Shipped 2026-05-15 |
| `no-dupe-keys` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-case` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-imports` | ✓ | Shipped 2026-05-15 (76/86 fixtures; type-only import combinations and the finer points of `includeExports` are still pending) |
| `no-empty-character-class` | ✓ | Shipped 2026-05-15 (hand-rolled regex pattern scanner — no full regex AST needed) |
| `no-empty-pattern` | ✓ | Shipped 2026-05-15 |
| `no-ex-assign` | ✓ | Shipped 2026-05-15 |
| `no-fallthrough` | ✓ | Shipped 2026-05-15 (86/86 fixtures) |
| `no-func-assign` | ✓ | Shipped 2026-05-15 |
| `no-import-assign` | ✓ | Shipped 2026-05-15 |
| `no-inner-declarations` | ✓ | Shipped 2026-05-15 (65/66 fixtures; one diverges on sourceType=module heuristics) |
| `no-invalid-regexp` | ✓ | Shipped 2026-05-15 (hand-rolled structural validator catches unbalanced parens, unclosed char classes, dangling backslash, unterminated named groups) |
| `no-irregular-whitespace` | ✓ | Shipped 2026-05-15 (220/220 fixtures) |
| `no-loss-of-precision` | ✓ | Shipped 2026-05-15 |
| `no-misleading-character-class` | ✓ | Shipped 2026-05-15 (flags astral-plane chars and surrogate pairs in classes without `u`/`v` flag) |
| `no-new-native-nonconstructor` | ✓ | Shipped 2026-05-15 |
| `no-obj-calls` | ✓ | Shipped 2026-05-15 |
| `no-promise-executor-return` | ✓ | Shipped 2026-05-15 |
| `no-prototype-builtins` | ✓ | Shipped 2026-05-15 |
| `no-self-assign` | ✓ | Shipped 2026-05-15 |
| `no-self-compare` | ✓ | Shipped 2026-05-15 |
| `no-setter-return` | ✓ | Shipped 2026-05-15 |
| `no-sparse-arrays` | ✓ | Shipped 2026-05-15 |
| `no-template-curly-in-string` | ✓ | Shipped 2026-05-15 |
| `no-this-before-super` | ✓ | Shipped 2026-05-15 (65/65 fixtures) |
| `no-undef` | ✓ | Shipped 2026-05-15 (87/97 fixtures; remaining cases need `globals`/`env` settings from oxc test sources, which the fixture extractor does not yet capture) |
| `no-unexpected-multiline` | ✓ | Shipped 2026-05-15 |
| `no-unmodified-loop-condition` | ✓ | Shipped 2026-05-15 (37/39 fixtures; remaining 2 cases need intra-procedural analysis of called functions) |
| `no-unreachable` | ✓ | Shipped 2026-05-15 (65/65 fixtures) |
| `no-unreachable-loop` | ✓ | Shipped 2026-05-15 (hand-written tests; no oxc source available) |
| `no-unsafe-finally` | ✓ | Shipped 2026-05-15 |
| `no-unsafe-negation` | ✓ | Shipped 2026-05-15 |
| `no-unsafe-optional-chaining` | ✓ | Shipped 2026-05-15 (82/82 fixtures, 100% — `with(...)` head detected via integer cast to KindWithStatement) |
| `no-unused-private-class-members` | ✓ | Shipped 2026-05-15 |
| `no-unused-vars` | ✓ | Shipped 2026-05-15 (conservative port — flags declarations whose name never appears in a reference position; underscore-prefix and `export`-attached names suppressed; full option set deferred) |
| `no-use-before-define` | ✓ | Shipped 2026-05-15 (239/354 fixtures; remaining cases need cross-scope analysis to honor the `variables: false` option fully) |
| `no-useless-backreference` | ✓ | Shipped 2026-05-15 (hand-rolled pattern scanner counts capturing/named groups, flags backreferences with no target) |
| `require-atomic-updates` | ✓ | Shipped 2026-05-15 (hand-written tests; no oxc source available) |
| `use-isnan` | ✓ | Shipped 2026-05-15 |
| `valid-typeof` | ✓ | Shipped 2026-05-15 |
