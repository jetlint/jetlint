# typescript-eslint compatibility overview

Summary of every type-aware rule's compatibility with upstream's
published test fixtures. Each rule has a vendored fixture under
`testdata/typescript-eslint/<rule>.test.ts` and a compatibility harness
under `internal/rules/<pkg>/tselintcompat_test.go`.

Run any single rule's harness with:

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/<pkg>/
```

## Implemented (≥80%)

| Rule | Score | Status |
|---|---:|---|
| no-floating-promises | 175/175 (100.0%) | full implementation, all options |
| no-array-delete | 29/29 (100.0%) | full implementation |
| no-base-to-string | 311/315 (98.7%) | full implementation, all options; 4 known limitations |
| no-unsafe-argument | 38/42 (90.5%) | implemented |
| only-throw-error | 78/89 (87.6%) | implemented |
| no-unsafe-unary-minus | 20/23 (87.0%) | implemented |
| no-for-in-array | 18/22 (81.8%) | implemented |
| no-meaningless-void-operator | 4/5 (80.0%) | implemented |

## Implemented (60–80%)

| Rule | Score | Status |
|---|---:|---|
| unbound-method | 141/202 (69.8%) | stub baseline |
| no-useless-default-assignment | 60/83 (72.3%) | stub baseline |
| restrict-template-expressions | 57/79 (72.2%) | stub baseline |
| related-getter-setter-pairs | 16/23 (69.6%) | stub baseline |
| no-unnecessary-template-expression | 56/71 (78.9%) | stub baseline |
| no-implied-eval | 48/70 (68.6%) | stub baseline |
| prefer-readonly-parameter-types | 89/130 (68.5%) | stub baseline |
| require-await | 37/54 (68.5%) | stub baseline |
| no-unnecessary-type-arguments | 48/71 (67.6%) | stub baseline |
| prefer-regexp-exec | 25/37 (67.6%) | stub baseline |
| await-thenable | 82/121 (67.8%) | implemented |
| consistent-return | 19/30 (63.3%) | stub baseline |
| dot-notation | 38/61 (62.3%) | stub baseline |
| prefer-promise-reject-errors | 101/161 (62.7%) | implemented |
| no-misused-promises | 132/215 (61.4%) | implemented (conditionals + spreads) |
| no-unsafe-call | 23/38 (60.5%) | implemented (any-typed callee) |

## Implemented (40–60%)

| Rule | Score | Status |
|---|---:|---|
| prefer-destructuring | 55/92 (59.8%) | stub baseline |
| no-unnecessary-type-parameters | 95/160 (59.4%) | stub baseline |
| no-mixed-enums | 30/51 (58.8%) | stub baseline |
| no-unnecessary-type-conversion | 38/66 (57.6%) | stub baseline |
| no-unnecessary-type-assertion | 127/223 (57.0%) | stub baseline |
| use-unknown-in-catch-callback-variable | 31/56 (55.4%) | stub baseline |
| no-unnecessary-condition | 163/296 (55.1%) | stub baseline |
| non-nullable-type-assertion-style | 11/20 (55.0%) | stub baseline |
| prefer-reduce-type-parameter | 17/31 (54.8%) | stub baseline |
| return-await | 51/95 (53.7%) | stub baseline |
| no-unsafe-return | 33/62 (53.2%) | implemented (any return) |
| no-redundant-type-constituents | 55/104 (52.9%) | stub baseline |
| switch-exhaustiveness-check | 54/104 (51.9%) | stub baseline |
| require-array-sort-compare | 17/33 (51.5%) | stub baseline |
| consistent-type-exports | 24/47 (51.1%) | stub baseline |
| strict-void-return | 105/210 (50.0%) | stub baseline |
| restrict-plus-operands | 59/119 (49.6%) | stub baseline |
| no-confusing-void-expression | 53/108 (49.1%) | stub baseline |
| promise-function-async | 26/53 (49.1%) | stub baseline |
| no-unnecessary-boolean-literal-compare | 22/45 (48.9%) | stub baseline |
| prefer-readonly | 79/162 (48.8%) | stub baseline |
| no-unsafe-enum-comparison | 41/85 (48.2%) | stub baseline |
| prefer-string-starts-ends-with | 58/123 (47.2%) | stub baseline |
| no-unnecessary-qualifier | 8/17 (47.1%) | stub baseline |
| no-unsafe-type-assertion | 7/15 (46.7%) | stub baseline |
| naming-convention | 41/88 (46.6%) | stub baseline |
| no-unsafe-member-access | 16/35 (45.7%) | implemented (any receiver) — bug TBD |
| strict-boolean-expressions | 97/214 (45.3%) | partial |
| prefer-nullish-coalescing | 275/617 (44.6%) | stub baseline |
| prefer-find | 19/45 (42.2%) | stub baseline |
| no-duplicate-type-constituents | 34/82 (41.5%) | stub baseline |
| prefer-optional-chain | 18/45 (40.0%) | stub baseline |

## Implemented (<40%)

| Rule | Score | Status |
|---|---:|---|
| no-unsafe-assignment | 36/91 (39.6%) | partial baseline |
| prefer-return-this-type | 8/21 (38.1%) | stub baseline |
| no-deprecated | 93/262 (35.5%) | stub baseline |
| no-misused-spread | 40/128 (31.2%) | stub baseline |
| prefer-includes | 13/42 (31.0%) | stub baseline |

## Per-rule docs

The implemented rules with notable work documented:
- `docs/TSEC-COMPAT-NO-FLOATING-PROMISES.md`
- `docs/TSEC-COMPAT-NO-BASE-TO-STRING.md`
- `docs/TSEC-COMPAT-NO-MISUSED-PROMISES.md`
- `docs/TSEC-COMPAT-NO-UNSAFE-ASSIGNMENT.md`
- `docs/TSEC-COMPAT-STRICT-BOOLEAN-EXPRESSIONS.md`

The other rules still ship as stubs whose harnesses provide the
baseline measurement. Implementing each one is an instance of the
loop in `docs/RULE-LOOP.md` (see commit history for the script's
evolution).

## Notes on the stub baseline

A no-op rule scores high on this metric because most upstream cases
are valid (no diagnostic expected). That doesn't mean the rule is
"already mostly working" — it means the implementation gap is
concentrated in the `invalid` half of each fixture. The
implementations marked above explicitly DO emit diagnostics for the
invalid cases they target.
