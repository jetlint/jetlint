# The rule-implementation loop

A 14-step loop for taking any typescript-eslint type-aware rule from
zero to high compatibility against upstream's published test fixtures.
Treat upstream as a black box — read the test fixtures as the spec,
not the upstream source.

1. Vendor the upstream rule's `.test.ts` file into
   `testdata/typescript-eslint/<rule>.test.ts`, with their LICENSE
   alongside.
2. Run the generic `tselintcompat` loader against it to extract
   `(code, valid?, expectedErrorCount, options)` cases. Confirm the
   count matches upstream's.
3. Wire the harness: for each case, write code to a temp project, load
   via wrapper, run the rule with extracted options, compare diagnostic
   count, log pass/fail.
4. Run the harness cold against an empty/stub rule to get the baseline
   percentage and a categorized failure list (false positives, false
   negatives, options-dependent).
5. Treat the upstream rule as a black box. For each failure cluster,
   read the failing test cases as the spec — what shape they share,
   what counts upstream expects — and reason about what semantic in
   your rule must change. Implement the change in Go using the wrapper
   API; add wrapper helpers (`pkg/checker/checker.go`) as new primitives
   are needed and guard panicky upstream APIs with safe wrappers.
6. Re-run the harness. Diff failures vs the previous run; investigate
   every regression before celebrating new passes. Iterate until
   no-options failures = 0.
7. Survey the test fixtures to enumerate every option key that appears
   (`grep "options:"`); that's the rule's option surface for the
   harness. Add the `Options` struct, `DefaultOptions()`,
   `NewWithOptions`, `OptionsFromJSON` — strict unknown-key rejection,
   both string and `{from, name}` matcher shapes where relevant.
8. Update the harness's `optionsFromCase` to translate extracted
   options to the typed `Options`. Re-run until 100%.
9. Add an arm in `cli.go`'s `buildRules` dispatcher so the rule reads
   options from `ResolvedConfig.RuleOptions`. Other rules without
   options-support continue to reject non-empty options.
10. Add unit tests for `OptionsFromJSON` (defaults, each field, unknown
    key, bad shape) and CLI integration tests proving
    `.tsgolintrc.json` actually changes rule behavior.
11. Run the full short suite to confirm no regressions in other rules.
12. Update `docs/TSEC-COMPAT-<rule>.md` with the headline number, what's
    recognized, what each option does, and the on-disk config example.
13. Commit the rule (lint repo via `jj describe`; fork repo via
    `git commit` if wrapper changed). One commit per rule.
14. Move on to the next rule.
