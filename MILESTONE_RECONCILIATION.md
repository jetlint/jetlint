# Milestone Reconciliation Report

Generated from open issues in milestones #2/#5/#6/#7 of `jetlint/jetlint` vs. rule packages under `internal/rules/`. A rule is "implemented" if a package directory matching the dash-stripped rule id exists AND contains both implementation `.go` and `_test.go` files. Full Go test suite is green on `feat/big-batch` (HEAD `kyrvzuxz`).

## Summary table

| Milestone | Open issues | Already implemented (close-ready) | Still missing |
|---|---|---|---|
| #2 suspicious   | 66 | 48 | 17 (+1 ambiguous: #512 `strict`) |
| #5 complexity   | 45 | 35 | 10 |
| #6 style        | 69 | 53 | 16 |
| #7 a11y         | 36 | 32 | 4  |
| **Totals**      | **216** | **168** | **47 (+1)** |

All 168 "implemented" packages were verified to have both a non-test `.go` file and a `_test.go` file. Issues for these can be closed immediately.

## Still missing — full lists

### #2 Suspicious (17 + 1 ambiguous)
#518 use-iterable-callback-return, #507 prefer-namespace-keyword, #504 no-useless-regex-backrefs, #502 no-unused-expressions, #497 no-unassigned-variables, #488 no-redeclare, #482 no-misused-new, #476 no-instanceof-array, #475 no-import-cycles, #464 no-exports-in-test, #462 no-evolving-types, #459 no-empty, #448 no-deprecated-imports, #441 no-confusing-void-type, #428 guard-for-in, #426 default-case-last, #425 adjacent-overload-signatures. Ambiguous: #512 `strict` (no kebab id in title — needs manual check).

### #5 Complexity (10)
#222 use-regex-literals, #221 use-optional-chain, #205 no-useless-undefined, #203 no-useless-this-alias, #197 no-useless-lone-block-statements, #195 no-useless-fragments, #194 no-useless-escape, #190 no-useless-constructor, #175 no-implicit-coercion, #169 no-excessive-cognitive-complexity.

### #6 Style (16)
#424 use-unified-type-signatures, #419 use-single-var-declarator, #415 use-readonly-class-properties, #408 use-naming-convention, #407 use-literal-enum-members, #403 use-filenaming-convention, #397 use-const, #393 use-consistent-curly-braces, #389 use-component-export-only-modules, #385 use-at-index, #382 prefer-template, #364 no-magic-numbers, #363 no-jsx-literals, #362 no-inferrable-types, #351 default-case, #350 consistent-type-imports.

### #7 A11y (4)
#151 use-heading-content, #146 use-aria-activedescendant-with-tabindex, #145 use-anchor-content, #144 use-alt-text.

## Top 5 highest-leverage missing rules per milestone

Leverage = frequency of real-world flag count + small surface area.

- **#2 suspicious**: #459 no-empty, #488 no-redeclare, #502 no-unused-expressions, #428 guard-for-in, #426 default-case-last.
- **#5 complexity**: #194 no-useless-escape, #190 no-useless-constructor, #221 use-optional-chain, #205 no-useless-undefined, #175 no-implicit-coercion.
- **#6 style**: #397 use-const, #382 prefer-template, #350 consistent-type-imports, #351 default-case, #362 no-inferrable-types.
- **#7 a11y**: #144 use-alt-text, #145 use-anchor-content, #151 use-heading-content, #146 use-aria-activedescendant-with-tabindex (only 4 missing — finish all of them).

## Notes / caveats

- Match heuristic: extracts kebab tokens (`no-foo-bar`) and camelCase tokens (`noFooBar` → `no-foo-bar`) from issue titles, then checks `internal/rules/<id-without-dashes>/`. False positives possible if a package exists for an unrelated rule with the same id; spot-check before bulk-closing.
- #512 `strict` did not yield a kebab id from the title — manual review needed.
- Recommend bulk-closing the 168 issues with a comment referencing `feat/big-batch` HEAD `kyrvzuxz` once the bookmark merges.
