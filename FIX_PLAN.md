# FIX_PLAN.md — running plan for the suspicious/complexity/style/a11y batch

Working bookmark: `feat/big-batch`. PR: opened against `main`.

Status snapshot (see `MILESTONE_RECONCILIATION.md` for derivation):

| Milestone | Open | Close-ready (impl exists) | Still missing |
|---|---:|---:|---:|
| #2 suspicious | 64 | 50 | 14 (+1 ambiguous #512 `strict`) |
| #5 complexity | 45 | 35 | 10 |
| #6 style | 69 | 53 | 16 |
| #7 a11y | 34 | 34 | 0 (wiring + closure only) |

**a11y status (2026-05-17): implementation + wiring 100%.** All 36 a11y
packages (16 `no-*` + 20 `use-*`, 33 open issues + 3 already landed) are
now registered in `internal/rules/registry.go` (new `CategoryA11y`
constant + 36 `Metadata` entries), listed in `additionalRulesSnapshot()`
in `registry_test.go`, and wired into `buildRules` in
`internal/cli/cli.go` so `.jetlintrc.json` can opt them in. The
`go test ./internal/rules/` registry suite passes. What remains is bulk
issue closure once PR #619 merges.

Discrepancy noted: FIX_PLAN previously said "34 open / 37 a11y rules";
actual head count is 33 open issues + 3 landed = 36 rules. The 34/37
figures in `MILESTONE_RECONCILIATION.md` and the survey table above
should be reconciled to 33/36 in a follow-up bookkeeping pass.

## Active work / blockers

- _(none)_ — pick from the next-up queue.

## Notes carried over (not regressions)

- `internal/format` tests `TestHumanFormatter_HeaderHasTrailingSeparatorBar`
  and `TestHumanFormatter_ColorAlwaysProducesANSIEscapes` are
  environment-dependent and fail under non-TTY sandboxes (NO_COLOR /
  isatty detection strips the `━` separator and ANSI escapes the
  tests expect unconditionally). They pass on local TTY runs but
  CI/sandbox runs see the failure. Pre-existing, orthogonal to rule
  work; the prior "resolved 2026-05-17" claim only held under a TTY.
  Real fix: make the formatter color-decision injectable so the tests
  set the desired mode directly rather than relying on env detection.
- `cli.go:432` carries a `errors.As(err, &te)` call that the linter
  suggests rewriting as `errors.AsType[*toolerr.Error]` (Go 1.26
  generics helper). Pre-existing; refactor in a separate commit so the
  current diff stays rule-scoped.

## Next-up queue (highest leverage first)

Priority order: finish #7 a11y (only 4 left) → highest-leverage suspicious →
complexity → style. Each entry resolves one rule via the
`docs/RULE-LOOP.md` 14-step procedure and one rule-scoped commit on
`feat/big-batch`.

### #7 a11y — IMPLEMENTATION + WIRING COMPLETE (36/36 packages); bulk closure remains

Survey (2026-05-17): every open milestone-#7 issue maps to an
`internal/rules/<pkg>/` package whose `EslintCompatibility` harness
passes at 100%. As of the wiring sweep on the same day, all 36 packages
(33 still-open issues + 3 already-landed rules) are registered and
wired into `cli.go`'s `buildRules`. `docs/RULE-CATEGORIES.md` carries a
new `a11y` row, the decision rubric is unblocked, and the "Growing
beyond type-aware rules" section reflects the 36-rule corpus.

One prerequisite remains:

1. **Bulk issue closure.** Once PR #619 merges, bulk-close all 33
   still-open a11y issues with a comment pointing to the merged commit
   hash and rule path. Each closure should include the harness pass-rate
   line so the record matches reality. Use the explicit issue→package
   mapping below; do not rely on directory-name globbing (it would
   miss #146 `use-aria-activedescendant-with-tabindex` → package
   `usearia`).

Open issue → package mapping (all 34 harnesses currently pass):

| Issue | Rule ID | Package |
|---|---|---|
| #128 | no-access-key | `noaccesskey` |
| #129 | no-aria-hidden-on-focusable | `noariahiddenonfocusable` |
| #130 | no-aria-unsupported-elements | `noariaunsupportedelements` |
| #131 | no-autofocus | `noautofocus` |
| #132 | no-distracting-elements | `nodistractingelements` |
| #133 | no-header-scope | `noheaderscope` |
| #134 | no-interactive-element-to-noninteractive-role | `nointeractiveelementtononinteractiverole` |
| #135 | no-label-without-control | `nolabelwithoutcontrol` |
| #136 | no-noninteractive-element-interactions | `nononinteractiveelementinteractions` |
| #137 | no-noninteractive-element-to-interactive-role | `nononinteractiveelementtointeractiverole` |
| #138 | no-noninteractive-tabindex | `nononinteractivetabindex` |
| #139 | no-positive-tabindex | `nopositivetabindex` |
| #140 | no-redundant-alt | `noredundantalt` |
| #141 | no-redundant-roles | `noredundantroles` |
| #142 | no-static-element-interactions | `nostaticelementinteractions` |
| #143 | no-svg-without-title | `nosvgwithouttitle` |
| #146 | use-aria-activedescendant-with-tabindex | `usearia` |
| #147 | use-aria-props-for-role | `useariapropsforrole` |
| #148 | use-aria-props-supported-by-role | `useariapropssupportedbyrole` |
| #149 | use-button-type | `usebuttontype` |
| #150 | use-focusable-interactive | `usefocusableinteractive` |
| #152 | use-html-lang | `usehtmllang` |
| #153 | use-iframe-title | `useiframetitle` |
| #154 | use-key-with-click-events | `usekeywithclickevents` |
| #155 | use-key-with-mouse-events | `usekeywithmouseevents` |
| #156 | use-media-caption | `usemediacaption` |
| #157 | use-semantic-elements | `usesemanticelements` |
| #158 | use-valid-anchor | `usevalidanchor` |
| #159 | use-valid-aria-props | `usevalidariaprops` |
| #160 | use-valid-aria-role | `usevalidariarole` |
| #161 | use-valid-aria-values | `usevalidariavalues` |
| #162 | use-valid-autocomplete | `usevalidautocomplete` |
| #163 | use-valid-lang | `usevalidlang` |

Note the non-obvious mapping: #146 `use-aria-activedescendant-with-tabindex`
lives in package `usearia` (not `useariaactivedescendantwithtabindex`).
A future loop survey by directory name will miss it; use this table.

Landed this batch (prior loops):
- #145 use-anchor-content — eslint a11y fixture 2/2 passes; rule at
  `internal/rules/useanchorcontent/`.
- #151 use-heading-content — eslint a11y fixture 2/2 passes; rule at
  `internal/rules/useheadingcontent/` (commit a5d2456 on
  feat/big-batch, PR #619). Closed 2026-05-17.
- use-alt-text — landed earlier in the batch; package
  `internal/rules/usealttext/`. No matching open issue under
  milestone #7 — confirm it's already closed before resuming.

### #2 suspicious — 12 remaining (top-leverage first)

1. #488 no-redeclare — **needs scope/binding analysis**; previously
   skipped on `feat/big-batch` (commit `wip(rules): more +6 (...,
   redeclare-skip)`). Defer until scope-symbol helpers land.
2. #518 use-iterable-callback-return
3. #507 prefer-namespace-keyword
4. #504 no-useless-regex-backrefs
5. #497 no-unassigned-variables
6. #482 no-misused-new
7. #476 no-instanceof-array
8. #475 no-import-cycles
9. #464 no-exports-in-test
10. #448 no-deprecated-imports
11. #441 no-confusing-void-type
12. #425 adjacent-overload-signatures
13. (ambiguous) #512 `strict` — needs manual title triage

_Landed 2026-05-17 in this batch:_
- #506 `no-with` — `internal/rules/nowith/`, 12/12 oxlint cases pass.
  Pure AST: dispatches on `KindWithStatement` (Kind=255 magic constant
  — wrapper does not re-export this Kind; precedent in
  `noconstantbinaryexpression` for binary operator tokens). No options.
  Wired into `cli.go` buildRules and the registry as
  `CategorySuspicious`.
- #428 `guard-for-in` — `internal/rules/guardforin/`, 12/12 oxlint cases
  pass. Pure AST: reports a `for-in` loop whose body is not guarded.
  Accepts EmptyStatement, IfStatement (direct), empty Block, Block with
  exactly one IfStatement, and Block whose first statement is
  `if (...) continue;` (raw or `{ continue; }`). No options.
- #502 `no-unused-expressions` — `internal/rules/nounusedexpressions/`,
  110/110 oxlint cases pass. Flags ExpressionStatements whose expression
  has no observable side effect (literals, identifiers, member access,
  comparisons, untagged templates). Recognises TS wrappers (`as`,
  `satisfies`, `<T>e`, `e!`, `e<T>`), unwraps Parenthesized and
  NonNullExpression, and treats Call/New/Await/Yield/Update/Delete/Void
  as side-effecting. Skips directive prologues in SourceFile, function
  bodies, and ModuleBlock — but not class static blocks, IfStatement
  bodies, or other free-standing blocks (matches oxlint). Options:
  allowShortCircuit, allowTernary, allowTaggedTemplates, enforceForJSX.
  Required wrapper bump: typescript-go v0.2.8 exposes KindJsxElement,
  KindJsxFragment, KindClassStaticBlockDeclaration, KindMetaProperty
  (commit cfbb0648a on jetlint/typescript-go).
- #426 `default-case-last` — `internal/rules/defaultcaselast/`, 37/37
  oxlint cases pass. Pure AST: reports any non-last `default` clause
  in a `switch` statement. No options.
- #462 `no-evolving-types` — `internal/rules/noevolvingtypes/`, 2/2
  biome cases pass. Flags `let`/`var`/`const` decls with no annotation
  whose initializer is missing, `null`, or `[]` (TypeScript would
  infer an evolving any). Skips for-in/for-of bindings and
  destructuring patterns.
- #459 `no-empty` — `internal/rules/noempty/`, 34/34 oxlint cases pass.
  Filters function/method bodies; comment-aware for blocks; switch
  reports unconditionally; `allowEmptyCatch` option supported.

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
  (camelCase → kebab conversion may yield false positives). For a11y,
  use the explicit issue→package table in the `#7 a11y` section above
  — directory-name globbing alone will miss #146 (`usearia`).
- **A11y registry + CLI wiring sweep — DONE 2026-05-17** on
  `feat/big-batch`. Added `CategoryA11y` constant, 36 `Metadata`
  entries in `All`, 36 imports + `pkg.New()` lines in `buildRules`,
  appended 36 IDs to `additionalRulesSnapshot()`, removed the "blocked
  on JSX support" caveat from `docs/RULE-CATEGORIES.md`, and added an
  `a11y (36)` section to the current rule assignments. Registry tests
  pass; the only failing tests in `go test ./...` are the pre-existing
  `internal/format` human-formatter failures noted above.
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
