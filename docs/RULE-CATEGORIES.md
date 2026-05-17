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

Of the 61 rules shipped today, the assignment is:

### correctness (30)

`await-thenable`, `consistent-return`, `no-array-delete`, `no-base-to-string`,
`no-dupe-keys`, `no-duplicate-case`, `no-floating-promises`, `no-for-in-array`,
`no-misused-promises`, `no-misused-spread`, `no-mixed-enums`, `no-self-assign`,
`no-self-compare`, `no-unsafe-argument`, `no-unsafe-assignment`,
`no-unsafe-call`, `no-unsafe-enum-comparison`, `no-unsafe-member-access`,
`no-unsafe-return`, `no-unsafe-unary-minus`, `only-throw-error`,
`prefer-promise-reject-errors`, `related-getter-setter-pairs`,
`require-array-sort-compare`, `require-await`, `strict-void-return`,
`switch-exhaustiveness-check`, `use-isnan`,
`use-unknown-in-catch-callback-variable`, `valid-typeof`

Non-type-aware rules in this category — `no-dupe-keys`, `no-duplicate-case`,
`no-self-assign`, `no-self-compare`, `use-isnan`, `valid-typeof` — carry
`RequiresTypeChecking: false`. They walk the AST without ever calling the
checker and rely on syntactic helpers like [Node.SourceText][srctext],
[Node.PropertyAccessName][pan], and [Node.LiteralText][lit].

[srctext]: https://pkg.go.dev/github.com/microsoft/typescript-go/pkg/checker#Node.SourceText
[pan]: https://pkg.go.dev/github.com/microsoft/typescript-go/pkg/checker#Node.PropertyAccessName
[lit]: https://pkg.go.dev/github.com/microsoft/typescript-go/pkg/checker#Node.LiteralText

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

### a11y (36)

`no-access-key`, `no-aria-hidden-on-focusable`,
`no-aria-unsupported-elements`, `no-autofocus`, `no-distracting-elements`,
`no-header-scope`, `no-interactive-element-to-noninteractive-role`,
`no-label-without-control`, `no-noninteractive-element-interactions`,
`no-noninteractive-element-to-interactive-role`,
`no-noninteractive-tabindex`, `no-positive-tabindex`,
`no-redundant-alt`, `no-redundant-roles`,
`no-static-element-interactions`, `no-svg-without-title`,
`use-alt-text`, `use-anchor-content`,
`use-aria-activedescendant-with-tabindex`, `use-aria-props-for-role`,
`use-aria-props-supported-by-role`, `use-button-type`,
`use-focusable-interactive`, `use-heading-content`, `use-html-lang`,
`use-iframe-title`, `use-key-with-click-events`,
`use-key-with-mouse-events`, `use-media-caption`,
`use-semantic-elements`, `use-valid-anchor`, `use-valid-aria-props`,
`use-valid-aria-role`, `use-valid-aria-values`,
`use-valid-autocomplete`, `use-valid-lang`

All a11y rules are JSX-syntactic and carry `RequiresTypeChecking: false`.
Default severity is `off` — opt in via `.jetlintrc.json`. The rules are
ported from `eslint-plugin-jsx-a11y` and Biome's a11y group; each has a
passing `EslintCompatibility` harness against the upstream Biome fixtures
under `biome-fixtures/`.

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
- **a11y**: 36 rules ported from `eslint-plugin-jsx-a11y` and Biome's
  a11y group; see the `a11y` section above.

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
