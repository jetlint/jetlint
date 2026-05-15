# ESLint "Possible Problems" porting backlog

The complete list of ESLint-core "Possible Problems" rules, in
alphabetical order. Working through these in order, one rule per PR.
Update this file as rules land or assumptions change.

**Status counts**

- Shipped: 28
- Remaining (AST-only): ~25
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
| `getter-return` | | AST walk: does the getter body always return? |
| `no-async-promise-executor` | ✓ | Shipped 2026-05-15 |
| `no-await-in-loop` | ✓ | Shipped 2026-05-15 |
| `no-class-assign` | ✓ | Shipped 2026-05-15 |
| `no-compare-neg-zero` | ✓ | Shipped 2026-05-15 |
| `no-cond-assign` | ✓ | Shipped 2026-05-15 |
| `no-const-assign` | ✓ | Shipped 2026-05-15 |
| `no-constant-binary-expression` | | AST + light type inference. |
| `no-constant-condition` | | AST: literal boolean condition. |
| `no-constructor-return` | ✓ | Shipped 2026-05-15 |
| `no-control-regex` | ⊘ | Regex AST. |
| `no-debugger` | ✓ | Shipped 2026-05-15 |
| `no-dupe-args` | | AST: duplicate parameter names. |
| `no-dupe-class-members` | ✓ | Shipped 2026-05-15 |
| `no-dupe-else-if` | ✓ | Shipped 2026-05-15 |
| `no-dupe-keys` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-case` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-imports` | | AST: duplicate `import` source strings. |
| `no-empty-character-class` | ⊘ | Regex AST. |
| `no-empty-pattern` | ✓ | Shipped 2026-05-15 |
| `no-ex-assign` | ✓ | Shipped 2026-05-15 |
| `no-fallthrough` | | AST walk: does each case end in break/return/throw? |
| `no-func-assign` | ✓ | Shipped 2026-05-15 |
| `no-import-assign` | | Scope: assigning to an imported binding. |
| `no-inner-declarations` | | AST: function/var declared inside a non-function block. |
| `no-invalid-regexp` | ⊘ | Regex AST. |
| `no-irregular-whitespace` | | AST: source-text scan for control-whitespace characters. |
| `no-loss-of-precision` | | AST: numeric literal whose precision is lost. |
| `no-misleading-character-class` | ⊘ | Regex AST. |
| `no-new-native-nonconstructor` | | AST: `new Symbol(...)` / `new BigInt(...)`. |
| `no-obj-calls` | ✓ | Shipped 2026-05-15 |
| `no-promise-executor-return` | ✓ | Shipped 2026-05-15 |
| `no-prototype-builtins` | ✓ | Shipped 2026-05-15 |
| `no-self-assign` | ✓ | Shipped 2026-05-15 |
| `no-self-compare` | ✓ | Shipped 2026-05-15 |
| `no-setter-return` | ✓ | Shipped 2026-05-15 |
| `no-sparse-arrays` | ✓ | Shipped 2026-05-15 |
| `no-template-curly-in-string` | ✓ | Shipped 2026-05-15 |
| `no-this-before-super` | | AST walk: `this` reference before `super()` in derived constructor. |
| `no-undef` | | Checker: undeclared identifier (TS catches; AST-only fallback). |
| `no-unexpected-multiline` | | AST: ASI ambiguity (call without semicolon then `(`). |
| `no-unmodified-loop-condition` | | AST + scope: loop test references unchanged variables. |
| `no-unreachable` | | AST walk: statements after return/throw/break/continue. |
| `no-unreachable-loop` | | AST walk: every path through the loop body exits. |
| `no-unsafe-finally` | | AST walk: return/throw/break/continue inside finally. |
| `no-unsafe-negation` | | AST: `!a in b`, `!a instanceof b`. |
| `no-unsafe-optional-chaining` | | AST: optional chain whose result must not be undefined. |
| `no-unused-private-class-members` | | Scope: unused `#field`. |
| `no-unused-vars` | | Scope: unused declarations. TS catches under noUnusedLocals. |
| `no-use-before-define` | | Scope: reference before declaration. |
| `no-useless-backreference` | ⊘ | Regex AST. |
| `require-atomic-updates` | | Scope + async: `x = await … x …` pattern. |
| `use-isnan` | ✓ | Shipped 2026-05-15 |
| `valid-typeof` | ✓ | Shipped 2026-05-15 |
