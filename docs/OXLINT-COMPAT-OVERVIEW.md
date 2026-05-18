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
| prefer-namespace-keyword | 10/10 (100%) |
| use-iterable-callback-return (biome) | 2/2 (100%) |
| no-confusing-void-type (biome) | 2/2 (100%) |
| no-misused-new | 19/19 (100%) |
| adjacent-overload-signatures | 64/64 (100%) |
| no-empty-source (biome) | 11/11 (100%) |
| no-exports-in-test (biome) | 5/5 (100%) |
| no-redundant-use-strict (biome) | 15/15 (100%) |
| no-deprecated-imports (biome) | 4/4 (100%) |
| no-unassigned-variables (biome) | 4/4 (100%) |
| no-useless-regex-backrefs (biome) | 3/3 (100%) |
| no-redeclare (biome) | 51/51 (100%) |
| no-import-cycles (biome) | 8/8 (100%) |

**Aggregate: 884/884 cases pass (100%)** across all option
combinations the upstream fixtures exercise. The `no-import-cycles`
row reflects a directory-layout fixture under
`testdata/biome/no-import-cycles/` rather than a flat `<rule>.json`,
because the rule is multi-file by nature: each case is one source
file inside a shared in-program directory and its expected diagnostic
count depends on the import edges that exist between siblings. The
harness mirrors biome's `<stem>.options.json` mechanic by running
the engine per file with the right `Options` value.

The `no-useless-regex-backrefs` row covers the biome variant of
`internal/rules/nouselessbackreference/`, which exposes a second
constructor (`NewBiome()`) reporting under the biome id. The biome
variant flags only circular self-references (per the ECMAScript
spec, `\N` past the group count is an octal escape and `\k<name>`
without a matching named group is literal text); the eslint variant
keeps the broader "non-existent group" check expected by
`testdata/eslint/no-useless-backreference.json` and the rule's
unit tests.

The six rules with options (\`valid-typeof\`, \`use-isnan\`,
\`no-self-assign\`, \`no-empty\`, \`no-unused-expressions\`,
\`no-import-cycles\`) expose the standard \`Options\` /
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
