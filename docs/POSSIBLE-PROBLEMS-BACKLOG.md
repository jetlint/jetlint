# ESLint "Possible Problems" porting backlog

The complete list of ESLint-core "Possible Problems" rules, in
alphabetical order. Working through these in order, one rule per PR.
Update this file as rules land or assumptions change.

**Status counts**

- Shipped: 12
- Remaining (AST-only): ~40
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
| `no-compare-neg-zero` | | AST: `x === -0` or similar. |
| `no-cond-assign` | | AST: assignment expression in `if`/`while`/`for`/`do-while` condition. |
| `no-const-assign` | | Scope: assigning to a `const` binding. |
| `no-constant-binary-expression` | | AST + light type inference. |
| `no-constant-condition` | | AST: literal boolean condition. |
| `no-constructor-return` | | AST: `return value` inside a class constructor body. |
| `no-control-regex` | ⊘ | Regex AST. |
| `no-debugger` | | AST: `DebuggerStatement`. |
| `no-dupe-args` | | AST: duplicate parameter names. |
| `no-dupe-class-members` | | AST: duplicate member names in a class body. |
| `no-dupe-else-if` | | AST: structural equality across else-if conditions. |
| `no-dupe-keys` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-case` | ✓ | Shipped 2026-05-15 |
| `no-duplicate-imports` | | AST: duplicate `import` source strings. |
| `no-empty-character-class` | ⊘ | Regex AST. |
| `no-empty-pattern` | | AST: `{}` or `[]` destructuring with no bindings. |
| `no-ex-assign` | | Scope: assigning to a `catch` clause's exception binding. |
| `no-fallthrough` | | AST walk: does each case end in break/return/throw? |
| `no-func-assign` | | Scope: assigning to a function declaration's name. |
| `no-import-assign` | | Scope: assigning to an imported binding. |
| `no-inner-declarations` | | AST: function/var declared inside a non-function block. |
| `no-invalid-regexp` | ⊘ | Regex AST. |
| `no-irregular-whitespace` | | AST: source-text scan for control-whitespace characters. |
| `no-loss-of-precision` | | AST: numeric literal whose precision is lost. |
| `no-misleading-character-class` | ⊘ | Regex AST. |
| `no-new-native-nonconstructor` | | AST: `new Symbol(...)` / `new BigInt(...)`. |
| `no-obj-calls` | | AST: `Math(...)` / `JSON(...)` / `Reflect(...)` etc. |
| `no-promise-executor-return` | | AST: `return value` in a `new Promise()` executor body. |
| `no-prototype-builtins` | | AST: `obj.hasOwnProperty(...)` / `obj.isPrototypeOf(...)`. |
| `no-self-assign` | ✓ | Shipped 2026-05-15 |
| `no-self-compare` | ✓ | Shipped 2026-05-15 |
| `no-setter-return` | | AST walk: `return value` in a setter (no return value allowed). |
| `no-sparse-arrays` | | AST: `[1, , 2]` — array literal with elisions. |
| `no-template-curly-in-string` | | AST: `"${x}"` literal-string containing template syntax. |
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
