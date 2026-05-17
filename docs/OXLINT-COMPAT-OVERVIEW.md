# oxlint compatibility overview

Per-rule compatibility for jetlint's ESLint-core ports, validated
against fixtures vendored from
[oxc](https://github.com/oxc-project/oxc)'s linter test cases. Each
rule has a JSON fixture under `testdata/eslint/<rule>.json` produced
by `cmd/oxlint-fixtures` from the corresponding oxc source file at
`crates/oxc_linter/src/rules/eslint/<rule>.rs`, and a harness under
`internal/rules/<pkg>/eslintcompat_test.go`.

## Why oxlint and not ESLint core

ESLint core does not publish a machine-readable fixture format — its
test cases live inline in mocha `describe`/`it` blocks. oxlint
re-implements ESLint-core rules in Rust and stores pass/fail vectors
as Rust source literals, which extract cleanly. Those vectors are a
superset of ESLint's own test cases plus oxlint-authored additions, so
matching oxlint matches ESLint behaviorally.

## Generating fixtures

```bash
git clone https://github.com/oxc-project/oxc /tmp/oxc
go run ./cmd/oxlint-fixtures \
  --oxc /tmp/oxc \
  --out testdata/eslint \
  --rule no-self-compare \
  --rule use-isnan \
  --rule valid-typeof \
  --rule no-duplicate-case \
  --rule no-dupe-keys \
  --rule no-self-assign
```

Each fixture records the oxc SHA it was generated from, so
regenerations are reproducible. Re-run when bumping the pinned SHA or
adding a new rule.

## Running a harness

```bash
go test -count=1 -run EslintCompatibility -v ./internal/rules/<pkg>/
```

Harnesses log mismatches and a final pass-rate line. They do not gate
on 100%; the score is a baseline that improves as rules sharpen.

## Current compatibility

| Rule | Score |
|---|---:|
| no-self-compare | 24/24 (100%) |
| no-duplicate-case | 30/30 (100%) |
| no-dupe-keys | 50/50 (100%) |
| valid-typeof | 60/60 (100%) |
| use-isnan | 208/208 (100%) |
| no-self-assign | 92/92 (100%) |
| no-empty | 34/34 (100%) |
| default-case-last | 37/37 (100%) |
| no-unused-expressions | 110/110 (100%) |
| guard-for-in | 12/12 (100%) |
| no-with | 12/12 (100%) |
| no-instanceof-array | 17/17 (100%) |

**Aggregate: 686/686 cases pass (100%)** across all option
combinations the upstream fixtures exercise.

The five rules with options (\`valid-typeof\`, \`use-isnan\`,
\`no-self-assign\`, \`no-empty\`, \`no-unused-expressions\`) expose the standard \`Options\` /
\`DefaultOptions\` / \`OptionsFromJSON\` / \`NewWithOptions\` surface
so user-supplied config in \`.jetlintrc.json\` is plumbed through.

## Adding a new rule

1. Ship the rule (package + unit tests + registry entry).
2. Add `--rule <id>` to the extractor invocation and regenerate
   `testdata/eslint/<id>.json`.
3. Copy an existing `eslintcompat_test.go` and swap the rule package
   import, the JSON path, and the rule ID strings.
4. Run the harness, note the baseline, file follow-up issues for
   significant gaps.
