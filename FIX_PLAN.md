# FIX_PLAN.md — running plan for the suspicious/complexity/style/a11y batch

Working bookmark: `feat/big-batch`. PR: opened against `main`.

Status snapshot (see `MILESTONE_RECONCILIATION.md` for derivation):

| Milestone | Open | Close-ready (impl exists) | Still missing |
|---|---:|---:|---:|
| #2 suspicious | 60 | 51 | 10 (+1 ambiguous #512 `strict`) |
| #5 complexity | 45 | 35 | 10 |
| #6 style | 69 | 53 | 16 |
| #7 a11y | 33 | 33 | 0 (wiring + closure only) |

**a11y status (2026-05-17): implementation + wiring 100%.** All 36 a11y
packages (16 `no-*` + 20 `use-*`, 33 open issues + 3 already landed) are
registered in `internal/rules/registry.go` (`CategoryA11y` constant + 36
`Metadata` entries), listed in `additionalRulesSnapshot()` in
`registry_test.go`, and wired into `buildRules` in `internal/cli/cli.go`
so `.jetlintrc.json` can opt them in. The `go test ./internal/rules/`
registry suite passes. What remains is bulk issue closure once PR #619
merges.

## Active work / blockers

_2026-05-17: Loop landed `feat(rules): wire no-global-is-finite
(suspicious)` (commit `f800aaab`, change `sztspwrq`). Package
`internal/rules/noglobalisfinite/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go` (alphabetically between
`noglobaldirnamefilename` and `noheadimportindocument`), the
`CategorySuspicious` Metadata in `registry.go`, and the snapshot line
in `registry_test.go`. Closed #470. `go test ./internal/rules/
./internal/cli/ ./internal/rules/noglobalisfinite/` all green.
Recovery note: the local bookmark had a divergent `rpxxuzns` (local
`85bca96e` vs remote `9e6b3dcd` for #469 no-global-assign) inherited
from the previous loop. Resolution: rebase the new change onto
`feat/big-batch@origin`, then `jj bookmark forget feat/big-batch` +
`jj bookmark track feat/big-batch@origin` to drop the local
divergent copy, then move the bookmark forward. Lesson: avoid
`--allow-backwards` (hook-blocked); forget+retrack is the clean
recovery for divergent bookmark/change pairs._

_2026-05-17: Loop landed `feat(rules): wire no-alert (suspicious)`
(commit `c3ab766e`, change `lzsurksp`). Package `internal/rules/noalert/`
already existed and its `EslintCompatibility` harness passed; this
commit added the import + buildRules entry in `cli.go`, the
`CategorySuspicious` Metadata in `registry.go`, and the snapshot line
in `registry_test.go`. Closed #429. `go test ./...` green except the
two pre-existing `internal/format` non-TTY failures noted below._

_2026-05-17: Loop landed `feat(rules): wire no-bitwise-operators
(suspicious)` (commit `d2467f09`, change `zuumoxox`). Package
`internal/rules/nobitwiseoperators/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go`, the `CategorySuspicious` Metadata in
`registry.go`, and the snapshot line in `registry_test.go`. Closed
#434. `go test ./internal/rules/... ./internal/cli/...` fully green._

_2026-05-17: Loop landed `feat(rules): wire no-catch-assign
(suspicious)` (commit `110d0e7a`, change `szwzwtwp`). Package
`internal/rules/nocatchassign/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go`, the `CategorySuspicious` Metadata in
`registry.go`, and the snapshot line in `registry_test.go`. Closed
#435. `go test ./internal/rules/ ./internal/cli/
./internal/rules/nocatchassign/` all green._

_2026-05-17: Loop landed `feat(rules): wire no-comment-text
(suspicious)` (commit `7914c406`, change `usmurowr`). Package
`internal/rules/nocommenttext/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go`, the `CategorySuspicious` Metadata in
`registry.go`, and the snapshot line in `registry_test.go`. Closed
#437. `go test ./internal/cli/ ./internal/rules/ ./internal/rules/nocommenttext/`
all green._

_2026-05-17: Loop landed `feat(rules): wire no-console (suspicious)`
(commit `56406e95`, change `zxryoumn`). Package
`internal/rules/noconsole/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go` (alphabetically between `noconfusingvoidtype`
and `noconstantbinaryexpression`), the `CategorySuspicious` Metadata in
`registry.go`, and the snapshot line in `registry_test.go`. Closed #442.
`go test ./internal/rules/ ./internal/cli/ ./internal/rules/noconsole/`
all green._

_2026-05-17: Loop landed `feat(rules): wire no-const-enum (suspicious)`
(commit `a4603006`, change `quukzoxk`). Package `internal/rules/noconstenum/`
already existed and its `EslintCompatibility` harness passed; this
commit added the import + buildRules entry in `cli.go` (alphabetically
between `noconstassign` and `noconstructorreturn`), the
`CategorySuspicious` Metadata in `registry.go`, and the snapshot line
in `registry_test.go`. Closed #443.
`go test ./internal/rules/ ./internal/cli/ ./internal/rules/noconstenum/`
all green._

_2026-05-17: Loop landed `feat(rules): wire no-document-cookie
(suspicious)` (commit `39dca067`, change `luwooqkk`). Package
`internal/rules/nodocumentcookie/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go` (alphabetically between `nodeprecated`
and `nodupeargs`), the `CategorySuspicious` Metadata in `registry.go`,
and the snapshot line in `registry_test.go`. Closed #449.
`go test ./internal/rules/ ./internal/cli/ ./internal/rules/nodocumentcookie/`
all green._

_2026-05-17: Loop landed `feat(rules): wire no-explicit-any (suspicious)`
(commit `8ba17f92`, change `usywvzlk`). Package
`internal/rules/noexplicitany/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go` (alphabetically between `noexassign` and
`nofallthrough`), the `CategorySuspicious` Metadata in `registry.go`,
and the snapshot line in `registry_test.go`. Closed #463.
`go test ./internal/cli/ ./internal/rules/ ./internal/rules/noexplicitany/`
all green._

_2026-05-17: Loop landed `feat(rules): wire no-extra-non-null-assertion
(suspicious)` (commit `8901620b`, change `wllrzxkr`). Package
`internal/rules/noextranonnullassertion/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go` (alphabetically between `noexplicitany`
and `nofallthrough`), the `CategorySuspicious` Metadata in `registry.go`,
and the snapshot line in `registry_test.go`. Closed #465.
`go test ./internal/cli/ ./internal/rules/ ./internal/rules/noextranonnullassertion/`
all green._

_2026-05-17: Loop landed `feat(rules): wire no-function-assign
(suspicious)` (commit `a249d5ea`, change `zpyuxuxk`). Package
`internal/rules/nofunctionassign/` already existed and its
`EslintCompatibility` harness passed (13/13 oxlint cases); this commit
added the import + buildRules entry in `cli.go` (between `nofuncassign`
and `noglobaldirnamefilename`), the `CategorySuspicious` Metadata in
`registry.go`, and the snapshot line in `registry_test.go`. Closed #468.
`go test ./internal/cli/ ./internal/rules/ ./internal/rules/nofunctionassign/`
all green. Note: `no-func-assign` (`nofuncassign`, `CategoryCorrectness`)
is a separate, pre-existing oxlint-named rule and was not affected. Also
considered #461 no-empty-source first but its fixture has 0 cases (the
package ships with empty Handlers) — wiring it would close the issue
with a no-op, so skipped. Flag in plan: noemptysource needs a real
implementation before #461 should close._

_2026-05-17: Loop landed `feat(rules): wire no-non-null-asserted-optional-chain
(suspicious)` (change `ywmpumwx`). Package
`internal/rules/nononnullassertedoptionalchain/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go` (alphabetically between `nonodejsmodules`
and `nononoctaldecimalescape`), the `CategorySuspicious` Metadata in
`registry.go`, and the snapshot line in `registry_test.go`. Closed #483.
`go test ./internal/cli/ ./internal/rules/ ./internal/rules/nononnullassertedoptionalchain/`
all green._

_2026-05-17: Loop landed `feat(rules): wire no-array-index-key
(suspicious)` (commit `9c536c4f`, change `rrllysov`). Package
`internal/rules/noarrayindexkey/` already existed and its
`EslintCompatibility` harness passed; this commit added the import +
buildRules entry in `cli.go`, the `CategorySuspicious` Metadata in
`registry.go`, and the snapshot line in `registry_test.go`. Closed
#431. `go test ./...` fully green (no failures observed under the
current sandbox; the `internal/format` non-TTY failures noted earlier
did not surface this run — likely TTY/env-dependent as documented)._

_2026-05-17: Surveyed wiring-only opportunities. **80+ open issues**
across the four milestones map to existing `internal/rules/<pkg>/`
packages that are NOT wired into `cli.go` or registered in
`registry.go`. Confirmed wiring-only candidates (package exists +
tests pass):_

- _#429 no-alert → `noalert` (LANDED 2026-05-17)_
- _#431 no-array-index-key → `noarrayindexkey` (LANDED 2026-05-17)_
- _#434 no-bitwise-operators → `nobitwiseoperators` (LANDED 2026-05-17)_
- _#435 no-catch-assign → `nocatchassign` (LANDED 2026-05-17)_
- _#437 no-comment-text → `nocommenttext` (LANDED 2026-05-17)_
- _#442 no-console → `noconsole` (LANDED 2026-05-17)_
- _#443 no-const-enum → `noconstenum` (LANDED 2026-05-17)_
- _#449 no-document-cookie → `nodocumentcookie` (LANDED 2026-05-17)_
- _#450 no-document-import-in-page → `nodocumentimportinpage` (LANDED 2026-05-17)_
- _#451 no-double-equals → `nodoubleequals` (LANDED 2026-05-17)_
- _#457 no-duplicate-jsx-props → `noduplicatejsxprops` (LANDED 2026-05-17)_
- _#458 no-duplicate-test-hooks → `noduplicatetesthooks` (LANDED 2026-05-17)_
- _#460 no-empty-interface → `noemptyinterface` (LANDED 2026-05-17)_
- _#461 no-empty-source → `noemptysource`_
- _#463 no-explicit-any → `noexplicitany` (LANDED 2026-05-17)_
- _#465 no-extra-non-null-assertion → `noextranonnullassertion` (LANDED 2026-05-17)_
- _#467 no-focused-tests → `nofocusedtests` (LANDED 2026-05-17)_
- _#468 no-function-assign → `nofunctionassign` (LANDED 2026-05-17)_
- _#469 no-global-assign → `noglobalassign` (LANDED 2026-05-17, commit `9e6b3dcd`)_
- _#470 no-global-is-finite → `noglobalisfinite` (LANDED 2026-05-17, commit `f800aaab`)_
- _#471 no-global-is-nan → `noglobalisnan` (LANDED 2026-05-17, commit `aeb03b44`)_
- _#472 no-head-import-in-document → `noheadimportindocument` (LANDED 2026-05-17)_
- _#473 no-implicit-any-let → `noimplicitanylet` (LANDED 2026-05-17, commit `90f07f81`)_
- _#478 no-label-var → `nolabelvar` (LANDED 2026-05-17)_
- _#480/#481 no-misplaced-assertion/-misrefactored-shorthand-assign_
- _#483 no-non-null-asserted-optional-chain → `nononnullassertedoptionalchain` (LANDED 2026-05-17)_
- _#484 no-octal-escape → `nooctalescape` (LANDED 2026-05-17)_
- _#486/#487 no-react-forward-ref/-react-specific-props_
- _#490 no-shadow-restricted-names → `noshadowrestrictednames` (LANDED 2026-05-17, commit `62791e66`)_
- _#491 no-skipped-tests → `noskippedtests` (LANDED 2026-05-17, commit `cce0597b`)_
- _#493 no-suspicious-semicolon-in-jsx → `nosuspicioussemicoloninjsx` (LANDED 2026-05-17, commit `b169cb9d`)_
- _#495/#496 no-then-property/-ts-ignore_
- _#499 no-unsafe-declaration-merging → `nounsafedeclarationmerging` (LANDED 2026-05-17, commit `b4f64e76`)_
- _#503 no-useless-escape-in-string → `nouselessescapeinstring`_
- _#505 no-var → `novar` (LANDED 2026-05-17, commit `8f23a0be`)_
- _#515/#516/#517/#519/#520/#521 use-await/-error-message/-google-font-display/-number-to-fixed-digits-argument/-static-response-methods/-strict-mode_
- _Plus 30+ open in milestones #5 (complexity) and #6 (style)._

_Future loops should clear this wiring-only queue before tackling
from-scratch ports. The per-rule cost is ~3 file edits + 1 test run
(see the no-alert loop). Verify each candidate has a passing
EslintCompatibility/BiomeCompatibility test first._

_2026-05-17: Loop landed `feat(rules): wire use-self-closing-elements
(style)` (commit `29a2045ef5e2`, change `lxytozmn`). Rule package
`useselfclosingelements/` was already implemented and passing its
EslintCompatibility harness; this commit added the import + buildRules
entry in `cli.go`, the `CategoryStyle` Metadata in `registry.go`, and
the snapshot line in `registry_test.go`. Closed #416. `go test ./...`
green (zero new failures; pre-existing `internal/format` non-TTY notes
unchanged).

**Repeatable pattern discovered:** many open issues in the four
milestones map to packages that already exist under
`internal/rules/<pkg>/` but are not yet wired into `cli.go`'s
`buildRules` or registered in `registry.go`. Future loops should
prioritise these wiring-only closes (one Metadata entry + one import
+ one `pkg.New()` call + one snapshot line) over from-scratch ports.
Candidate sweep query: `ls internal/rules/` and diff against
`grep -E "rules/[a-z]+" cli.go | sort -u`._

**Recovery note (this loop):** the FIX_PLAN edit was initially made
in the same `@` as the just-pushed wiring commit, which rewrote the
pushed commit_id (29a2045 → 0682774f) and made push refuse with a
sideways-push error. Recovery: `jj restore --from @-` on FIX_PLAN.md
to revert the doc edit out of the pushed change, then
`jj new feat/big-batch@origin -m "docs(plan): ..."` to start a
fresh child commit, then `jj bookmark set feat/big-batch -r <remote>
--allow-backwards` to align the local bookmark with the remote
commit_id. The local divergent commit_id (0682774f) is harmless once
the bookmark no longer points at it. **Lesson for future loops:**
treat each rule/feature commit as immediately frozen after push;
plan-update commits MUST live in a separate `jj new` change.

_None blocking. The orphan divergence noted in earlier revisions of this plan
(local `69216b6494e7` vs pushed `90d480e63778` for change `zrusspwm`)
is resolved as of 2026-05-17: local + remote `feat/big-batch` are both
at `mwszlwsz 2b8ded95` (`feat(rules): add no-confusing-void-type`).
The recovery commit (change `zvoqrlsz`,
`feat(rules): add use-iterable-callback-return`) was successfully
inserted between the pushed `zrusspwm` tip and the new
no-confusing-void-type commit, and the whole chain has been pushed
fast-forward. Bookkeeping note: every new loop must still run
`jj new feat/big-batch -m "..."` as its first command — that
discipline is what kept the recovery clean._

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

### #2 suspicious — 5 remaining (top-leverage first; #493 wiring landed this loop, see Active work)

1. #504 no-useless-regex-backrefs — biome rule. Reports backreferences
   inside a regex literal that can only ever match the empty string.
2. #497 no-unassigned-variables — biome rule. `let`/`var` declarations
   without an initializer that are never assigned later in scope.
3. #475 no-import-cycles — biome rule. Cross-file analysis (needs
   module-graph plumbing); higher cost.
4. #464 no-exports-in-test — biome rule. Reports `export` statements in
   files matched by a test glob. Needs a test-file matcher.
5. #448 no-deprecated-imports — biome rule. Detects imports of symbols
   annotated `@deprecated` in their declaration site (type-aware).

Deferred:
- #488 no-redeclare — needs scope/binding analysis; previously
  skipped on `feat/big-batch` (commit `wip(rules): more +6 (...,
  redeclare-skip)`). Defer until scope-symbol helpers land.
- (ambiguous) #512 `strict` — needs manual title triage; oxc has no
  obvious kebab match.

_Landed 2026-05-17 in this batch:_
- #425 `adjacent-overload-signatures` —
  `internal/rules/adjacentoverloadsignatures/`, 64/64 oxlint cases pass.
  Pure AST. Registers KindSourceFile, KindBlock, KindModuleBlock,
  KindClassDeclaration/Expression, KindInterfaceDeclaration, and
  KindTypeLiteral. For each container, walks ForEachChild and
  classifies every direct child as a `*method` (or nil) by name+
  static-ness+call-signature+name-kind, then reports when a
  same-identity method appears with anything between it and the
  previous same-identity occurrence. Computed keys whose inner
  expression is a literal (`['foo']`, `[42]`) are unwrapped so they
  collide with their bare-name siblings — matches oxc's `static_name`
  semantics. Required an extractor fix in `cmd/oxlint-fixtures`: the
  tokenizer treated Rust raw identifiers like `r#static` as the start
  of a raw string `r#"..."#`; new `isRawStringStart` helper peeks past
  the `#` run and only enters raw-string mode when a `"` follows.
- #482 `no-misused-new` — `internal/rules/nomisusednew/`, 19/19 oxlint
  cases pass. Pure AST: hooks `KindInterfaceDeclaration` (reports
  `KindConstructSignature` whose return TypeReference name matches the
  interface name — covers `interface G { new <T>(): G<T>; }` since
  `TypeReferenceTypeName` returns the unqualified `G`),
  `KindClassDeclaration` / `KindClassExpression` (reports body-less
  `KindMethodDeclaration` named `new` whose return type references the
  class name), and `KindMethodSignature` everywhere (reports name
  `constructor` in interface bodies and type literals — `type T = {
  constructor(): void }` is caught universally without a parent check).
  Anonymous class expressions are skipped because their first child is
  not an Identifier so `declarationName` returns "".
- #507 `prefer-namespace-keyword` —
  `internal/rules/prefernamespacekeyword/`, 10/10 oxlint cases pass.
  Pure AST syntactic check: parses the leading keyword from the
  ModuleDeclaration's source text (after stripping `export`/`declare`
  modifiers, line and block comments, and whitespace) and reports
  when it equals `module`. Skips string-named ambient modules
  (`declare module 'foo'`), `declare global`, and inner segments of
  qualified `module A.B.C {}` (where the direct parent is itself a
  TSModuleDeclaration — mirrors oxc's skip). No options.
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
- #476 `no-instanceof-array` — `internal/rules/noinstanceofarray/`,
  17/17 oxlint unicorn cases pass. Flags `x instanceof Array` (right
  operand identifier `Array` after stripping parens). Operator extracted
  via source-span helper (`KindInstanceOfKeyword` not exposed). Required
  a `--plugin` flag added to `cmd/oxlint-fixtures` so unicorn rules
  extract.
- #518 `use-iterable-callback-return` —
  `internal/rules/useiterablecallbackreturn/`, 2/2 biome cases pass.
  Wraps `arraycallbackreturn` for non-`forEach` methods (semantics agree
  with biome) and overlays a looser `forEach` check: only an explicit
  `return <non-void expr>` (or non-void concise-body arrow) is flagged;
  bare returns, `void <expr>` returns, throw-only paths, and empty
  bodies are accepted. Default `checkForEach: true`. Follow-up bug:
  `cmd/biome-fixtures` only ingests `valid.js`/`invalid.js`, so the
  option-variant fixtures `checkForEachTrue.js`/`checkForEachFalse.js`
  (with `.options.json` sidecars) are not captured. Fix: teach the
  extractor to enumerate every `*.js`/`.options.json` pair under the
  rule directory and emit one case per file with sidecar options merged.
- #441 `no-confusing-void-type` —
  `internal/rules/noconfusingvoidtype/`, 2/2 biome cases pass. Pure AST:
  dispatches on `KindVoidKeyword` and inspects the direct parent.
  Reports `void` in Parameter (non-`this`), PropertyDeclaration/Signature,
  VariableDeclaration, TypeAliasDeclaration, TypeOperator (keyof void),
  ArrayType/Tuple/Rest, IntersectionType, MappedType value,
  TypeAssertion/As/Satisfies targets, and as a TypeParameter constraint
  (default position is permitted via identity match against
  `TypeParameterDefaultType()`). UnionType is left unflagged so
  `void | Promise<void>` doesn't false-positive.

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
