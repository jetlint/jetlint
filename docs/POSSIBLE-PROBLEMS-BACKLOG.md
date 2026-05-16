# ESLint "Possible Problems" porting backlog

The complete list of ESLint-core "Possible Problems" rules, in
alphabetical order. Working through these in order, one rule per PR.
Update this file as rules land or assumptions change.

**Status counts**

- Shipped: 57 (all rules have implementations; only 1 fixture-data conflict remains, in oxc itself)
- Remaining (AST-only): 0
- Remaining (needs regex AST infra): 0
- Partials needing 100%: no-use-before-define (353/354 — 1 fixture conflict in oxc itself)

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
| `no-constant-binary-expression` | ✓ | Shipped 2026-05-15 (251/251 fixtures, 100%; type-tag-aware equality, fresh-reference + well-known-builtin detection, array-coercion fixed cases, singleton self-equality across numeric/string/bigint literals, `??` chain non-nullish propagation) |
| `no-constant-condition` | ✓ | Shipped 2026-05-15 (306/306 fixtures, 100%; expanded constant-folding, scope-aware `undefined` / `Boolean` shadowing, array `+` string coercion, generator-yield-aware loop exemption, and array-spread template-span truthiness) |
| `no-constructor-return` | ✓ | Shipped 2026-05-15 |
| `no-control-regex` | ✓ | Shipped 2026-05-15 (hand-rolled pattern scanner with JS string-escape unescaping; covers `/.../` and `RegExp(string)` forms) |
| `no-debugger` | ✓ | Shipped 2026-05-15 |
| `no-dupe-args` | ✓ | Shipped 2026-05-15 (hand-written tests; no oxc source available) |
| `no-dupe-class-members` | ✓ | Shipped 2026-05-15 |
| `no-dupe-else-if` | ✓ | Shipped 2026-05-15 |
| `no-dupe-keys` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-case` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-imports` | ✓ | Shipped 2026-05-15 (86/86 fixtures, 100%; distinguishes anonymous `export *` from `export * as X`, treats side-effect-only imports as subsumed, type-only default+named pairs as semantically separate) |
| `no-empty-character-class` | ✓ | Shipped 2026-05-15 (hand-rolled regex pattern scanner — no full regex AST needed) |
| `no-empty-pattern` | ✓ | Shipped 2026-05-15 |
| `no-ex-assign` | ✓ | Shipped 2026-05-15 |
| `no-fallthrough` | ✓ | Shipped 2026-05-15 (86/86 fixtures) |
| `no-func-assign` | ✓ | Shipped 2026-05-15 |
| `no-import-assign` | ✓ | Shipped 2026-05-15 |
| `no-inner-declarations` | ✓ | Shipped 2026-05-15 (66/66 fixtures, 100%; nested bare-block chains rooted at a function body are recognised as in-scope under `blockScopedFunctions: allow`) |
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
| `no-undef` | ✓ | Shipped 2026-05-15 (97/97 fixtures, 100%; declaration-vs-initializer disambiguation, destructuring-assignment-shorthand detection, per-case lib (esnext + optional dom when env.browser is set), type-position skip, ambient `declare var` injection driven by the fixture's `globals` settings — extractor now captures the 3rd-tuple settings position) |
| `no-unexpected-multiline` | ✓ | Shipped 2026-05-15 |
| `no-unmodified-loop-condition` | ✓ | Shipped 2026-05-15 (39/39 fixtures, 100%; nested function-likes are skipped (only executed when called) and direct calls follow the callee's symbol into its body for cross-procedural mutation detection) |
| `no-unreachable` | ✓ | Shipped 2026-05-15 (65/65 fixtures) |
| `no-unreachable-loop` | ✓ | Shipped 2026-05-15 (hand-written tests; no oxc source available) |
| `no-unsafe-finally` | ✓ | Shipped 2026-05-15 |
| `no-unsafe-negation` | ✓ | Shipped 2026-05-15 |
| `no-unsafe-optional-chaining` | ✓ | Shipped 2026-05-15 (82/82 fixtures, 100% — `with(...)` head detected via integer cast to KindWithStatement) |
| `no-unused-private-class-members` | ✓ | Shipped 2026-05-15 |
| `no-unused-vars` | ✓ | Shipped 2026-05-15 (conservative port — flags declarations whose name never appears in a reference position; underscore-prefix and `export`-attached names suppressed; full option set deferred) |
| `no-use-before-define` | ✓ | Shipped 2026-05-15 (353/354 fixtures, 99.7%; flags self-init / TDZ patterns; eager class contexts override hoisting options when not nested in a deferred body; `new X()` at non-deferred positions forces flagging regardless of Classes option; `with (...)` blocks fall back to source-file binding lookup; declarationKind covers Enums, Modules, and Type Aliases (not Interfaces); typescript-eslint `enums: false` option suppresses forward refs to enum declarations; Variables option's deferred-body suppression is scoped to deferred bodies that do not enclose the declaration; AllowNamedExports and IgnoreTypeReferences honored. Remaining case is a fixture conflict: `"use strict"; a(); { function a() {} }` appears with both valid and invalid expectations in oxc's source) |
| `no-useless-backreference` | ✓ | Shipped 2026-05-15 (hand-rolled pattern scanner counts capturing/named groups, flags backreferences with no target) |
| `require-atomic-updates` | ✓ | Shipped 2026-05-15 (hand-written tests; no oxc source available) |
| `use-isnan` | ✓ | Shipped 2026-05-15 |
| `valid-typeof` | ✓ | Shipped 2026-05-15 |
