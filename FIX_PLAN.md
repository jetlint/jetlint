# FIX_PLAN.md — running plan for the suspicious/complexity/style/a11y batch

Working bookmark: `feat/big-batch`. PR: opened against `main`.

Status snapshot (see `MILESTONE_RECONCILIATION.md` for derivation):

| Milestone | Open | Close-ready (impl exists) | Still missing |
|---|---:|---:|---:|
| #2 suspicious | 66 | 48 | 17 (+1 ambiguous #512 `strict`) |
| #5 complexity | 45 | 35 | 10 |
| #6 style | 69 | 53 | 16 |
| #7 a11y | 36 | 33 | 3 |

## Active work / blockers

- _(none)_ — pick from the next-up queue.

## Next-up queue (highest leverage first)

Priority order: finish #7 a11y (only 4 left) → highest-leverage suspicious →
complexity → style. Each entry resolves one rule via the
`docs/RULE-LOOP.md` 14-step procedure and one rule-scoped commit on
`feat/big-batch`.

### #7 a11y — 3 remaining

1. #145 use-anchor-content
2. #151 use-heading-content
3. #146 use-aria-activedescendant-with-tabindex

### #2 suspicious — 17 remaining (top-leverage first)

1. #459 no-empty
2. #488 no-redeclare
3. #502 no-unused-expressions
4. #428 guard-for-in
5. #426 default-case-last
6. #518 use-iterable-callback-return
7. #507 prefer-namespace-keyword
8. #504 no-useless-regex-backrefs
9. #497 no-unassigned-variables
10. #482 no-misused-new
11. #476 no-instanceof-array
12. #475 no-import-cycles
13. #464 no-exports-in-test
14. #462 no-evolving-types
15. #448 no-deprecated-imports
16. #441 no-confusing-void-type
17. #425 adjacent-overload-signatures
18. (ambiguous) #512 `strict` — needs manual title triage

### #5 complexity — 10 remaining

1. #194 no-useless-escape
2. #190 no-useless-constructor
3. #221 use-optional-chain
4. #205 no-useless-undefined
5. #175 no-implicit-coercion
6. #222 use-regex-literals
7. #203 no-useless-this-alias
8. #197 no-useless-lone-block-statements
9. #195 no-useless-fragments
10. #169 no-excessive-cognitive-complexity

### #6 style — 16 remaining

1. #397 use-const
2. #382 prefer-template
3. #350 consistent-type-imports
4. #351 default-case
5. #362 no-inferrable-types
6. #424 use-unified-type-signatures
7. #419 use-single-var-declarator
8. #415 use-readonly-class-properties
9. #408 use-naming-convention
10. #407 use-literal-enum-members
11. #403 use-filenaming-convention
12. #393 use-consistent-curly-braces
13. #389 use-component-export-only-modules
14. #385 use-at-index
15. #364 no-magic-numbers
16. #363 no-jsx-literals

## Bookkeeping queue

- **Bulk-close 168 already-implemented issues** with a comment referencing
  `feat/big-batch` HEAD once the PR merges. Spot-check ambiguous ids
  (camelCase → kebab conversion may yield false positives).
- **Squash the ~24 in-flight `wip(rules): ...` commits** on `feat/big-batch`
  into rule-scoped conventional commits as part of PR finalization.
- **Manual triage** of #512 `strict` — title carries no kebab id.
- **Site sync**: when each rule lands at 100%, mirror to
  `jetlint.github.io/` per AGENT.md.

## Notes

- The 14-step loop in `docs/RULE-LOOP.md` is the source of truth for
  per-rule work. Vendor upstream fixtures first; never relax tests.
- Use parallel subagents only for read-only fixture surveys; serialize
  Go builds and `go test ./...` runs.
