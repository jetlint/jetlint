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
| **style**       | Formatting, naming, ordering. Pure preference; team-configurable.                                | off (opt-in)     |
| **nursery**     | New or iterating rules. May change shape or move to another group. Not in `recommended`.         | off              |

Reserved for later:

- **a11y** — accessibility rules. Blocked on JSX support.

Deliberately omitted (for now):

- **pedantic** / **restriction** (Clippy-style) — opinionated or contradictory
  rules. Fold them into `style` and ship off-by-default until a real need
  appears.

## Decision rubric

Apply in order; the first match wins:

1. **Wrong at runtime, or definitely a bug?** → `correctness`
2. **Enables injection, eval, prototype pollution, unsafe deserialization?** → `security`
3. **Pure perf cost with a known fast alternative?** → `performance`
4. **JSX accessibility?** → `a11y` (future)
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

Of the 61 rules shipped today, the assignment is:

### correctness (25)

`await-thenable`, `consistent-return`, `no-array-delete`, `no-base-to-string`,
`no-floating-promises`, `no-for-in-array`, `no-misused-promises`,
`no-misused-spread`, `no-mixed-enums`, `no-self-compare`, `no-unsafe-argument`,
`no-unsafe-assignment`, `no-unsafe-call`, `no-unsafe-enum-comparison`,
`no-unsafe-member-access`, `no-unsafe-return`, `no-unsafe-unary-minus`,
`only-throw-error`, `prefer-promise-reject-errors`,
`related-getter-setter-pairs`, `require-array-sort-compare`, `require-await`,
`strict-void-return`, `switch-exhaustiveness-check`,
`use-unknown-in-catch-callback-variable`

`no-self-compare` is the first non-type-aware rule: `RequiresTypeChecking:
false`. It uses [Node.SourceText][srctext] to compare operand source spans,
no checker required.

[srctext]: https://pkg.go.dev/github.com/microsoft/typescript-go/pkg/checker#Node.SourceText

### suspicious (9)

`no-confusing-void-expression`, `no-deprecated`, `no-unsafe-type-assertion`,
`promise-function-async`, `restrict-plus-operands`,
`restrict-template-expressions`, `return-await`, `strict-boolean-expressions`,
`unbound-method`

### complexity (17)

`no-duplicate-type-constituents`, `no-redundant-type-constituents`,
`no-unnecessary-boolean-literal-compare`, `no-unnecessary-condition`,
`no-unnecessary-qualifier`, `no-unnecessary-template-expression`,
`no-unnecessary-type-arguments`, `no-unnecessary-type-assertion`,
`no-unnecessary-type-conversion`, `no-unnecessary-type-parameters`,
`no-useless-default-assignment`, `non-nullable-type-assertion-style`,
`prefer-destructuring`, `prefer-nullish-coalescing`, `prefer-optional-chain`,
`prefer-reduce-type-parameter`, `prefer-return-this-type`

### performance (4)

`prefer-find`, `prefer-includes`, `prefer-regexp-exec`,
`prefer-string-starts-ends-with`

### security (1)

`no-implied-eval`

### style (6)

`consistent-type-exports`, `dot-notation`, `naming-convention`,
`no-meaningless-void-operator`, `prefer-readonly`,
`prefer-readonly-parameter-types`

### nursery (0)

None yet. Reserved for new rules during incubation.

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

## Growing beyond type-aware rules

When jetlint expands past the type-aware rule set, expected weight by group:

- **security** — the biggest current gap. Source plugins to mine:
  `eslint-plugin-security` (no-eval, no-new-func, detect-object-injection),
  `eslint-plugin-no-secrets`, `eslint-plugin-node-security`. About 10-15
  high-value rules. Security and suspicious deliver the most value per
  rule-of-effort because they catch bugs `tsc --noEmit` and Prettier cannot.
- **suspicious** without types: `no-self-compare`, `no-unreachable`,
  `no-constant-condition`, `no-dupe-keys`, `no-empty`, `no-fallthrough`.
- **complexity** without types: `prefer-const`, `no-var`, `no-unused-vars`,
  `no-useless-rename`, `no-useless-concat`.
- **style** without types: Biome and Prettier already own this turf. Low
  priority unless something is genuinely missing.
- **a11y**: blocked on JSX support; port from `eslint-plugin-jsx-a11y`
  once JSX is in.

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
