# ESLint "Possible Problems" porting backlog

The complete list of ESLint-core "Possible Problems" rules, in
alphabetical order. Working through these in order, one rule per PR.
Update this file as rules land or assumptions change.

**Status counts**

- Shipped: 36
- Remaining (AST-only): ~17
- Remaining (needs regex AST infra): 6

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
| `no-constant-binary-expression` | | AST + light type inference. |
| `no-constant-condition` | ✓ | Shipped 2026-05-15 (conservative port, 206/306 fixtures; full constant-folding deferred to a shared utility) |
| `no-constructor-return` | ✓ | Shipped 2026-05-15 |
| `no-control-regex` | ⊘ | Regex AST. |
| `no-debugger` | ✓ | Shipped 2026-05-15 |
| `no-dupe-args` | | AST: duplicate parameter names. |
| `no-dupe-class-members` | ✓ | Shipped 2026-05-15 |
| `no-dupe-else-if` | ✓ | Shipped 2026-05-15 |
| `no-dupe-keys` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-case` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-imports` | ✓ | Shipped 2026-05-15 (76/86 fixtures; type-only import combinations and the finer points of `includeExports` are still pending) |
| `no-empty-character-class` | ⊘ | Regex AST. |
| `no-empty-pattern` | ✓ | Shipped 2026-05-15 |
| `no-ex-assign` | ✓ | Shipped 2026-05-15 |
| `no-fallthrough` | ✓ | Shipped 2026-05-15 (86/86 fixtures) |
| `no-func-assign` | ✓ | Shipped 2026-05-15 |
| `no-import-assign` | ✓ | Shipped 2026-05-15 |
| `no-inner-declarations` | ✓ | Shipped 2026-05-15 (65/66 fixtures; one diverges on sourceType=module heuristics) |
| `no-invalid-regexp` | ⊘ | Regex AST. |
| `no-irregular-whitespace` | ✓ | Shipped 2026-05-15 (220/220 fixtures) |
| `no-loss-of-precision` | ✓ | Shipped 2026-05-15 |
| `no-misleading-character-class` | ⊘ | Regex AST. |
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
| `no-undef` | | Checker: undeclared identifier (TS catches; AST-only fallback). |
| `no-unexpected-multiline` | ✓ | Shipped 2026-05-15 |
| `no-unmodified-loop-condition` | | AST + scope: loop test references unchanged variables. |
| `no-unreachable` | ✓ | Shipped 2026-05-15 (65/65 fixtures) |
| `no-unreachable-loop` | | AST walk: every path through the loop body exits. |
| `no-unsafe-finally` | ✓ | Shipped 2026-05-15 |
| `no-unsafe-negation` | ✓ | Shipped 2026-05-15 |
| `no-unsafe-optional-chaining` | ✓ | Shipped 2026-05-15 (80/82 fixtures; `with(...)` not detected because `KindWithStatement` is unexported by the wrapper) |
| `no-unused-private-class-members` | ✓ | Shipped 2026-05-15 |
| `no-unused-vars` | | Scope: unused declarations. TS catches under noUnusedLocals. |
| `no-use-before-define` | | Scope: reference before declaration. |
| `no-useless-backreference` | ⊘ | Regex AST. |
| `require-atomic-updates` | | Scope + async: `x = await … x …` pattern. |
| `use-isnan` | ✓ | Shipped 2026-05-15 |
| `valid-typeof` | ✓ | Shipped 2026-05-15 |
