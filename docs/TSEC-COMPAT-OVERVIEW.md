# typescript-eslint compatibility overview

Summary of every type-aware rule's compatibility with upstream's
published test fixtures. Each rule has a vendored fixture under
`testdata/typescript-eslint/<rule>.test.ts` and a compatibility harness
under `internal/rules/<pkg>/tselintcompat_test.go`.

Run any single rule's harness with:

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/<pkg>/
```

Run them all in parallel:

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/...
```

## ≥ 80% — production-ready or close

| Rule | Score | Status |
|---|---:|---|
| no-floating-promises | 175/175 (100.0%) | full impl, all options, default-on (MVP) |
| no-array-delete | 29/29 (100.0%) | full impl |
| no-base-to-string | 311/315 (98.7%) | full impl, all options, default-on (MVP); 4 known limits |
| no-unsafe-argument | 38/42 (90.5%) | impl |
| only-throw-error | 78/89 (87.6%) | impl |
| no-unsafe-unary-minus | 20/23 (87.0%) | impl |
| no-for-in-array | 18/22 (81.8%) | impl |
| no-unnecessary-boolean-literal-compare | 36/45 (80.0%) | impl |
| no-meaningless-void-operator | 4/5 (80.0%) | impl |

## 60% – 80% — partial implementations and high-baseline stubs

| Rule | Score | Status |
|---|---:|---|
| no-unnecessary-template-expression | 56/71 (78.9%) | stub |
| no-unsafe-enum-comparison | 62/85 (72.9%) | impl |
| no-useless-default-assignment | 60/83 (72.3%) | stub |
| restrict-template-expressions | 57/79 (72.2%) | stub |
| unbound-method | 141/202 (69.8%) | stub |
| related-getter-setter-pairs | 16/23 (69.6%) | stub |
| no-implied-eval | 48/70 (68.6%) | stub |
| require-await | 37/54 (68.5%) | stub |
| prefer-readonly-parameter-types | 89/130 (68.5%) | stub |
| await-thenable | 82/121 (67.8%) | impl (constraint-walking) |
| no-unnecessary-type-arguments | 48/71 (67.6%) | stub |
| prefer-regexp-exec | 25/37 (67.6%) | stub |
| consistent-return | 19/30 (63.3%) | stub |
| prefer-promise-reject-errors | 101/161 (62.7%) | impl |
| dot-notation | 38/61 (62.3%) | stub |
| promise-function-async | 33/53 (62.3%) | impl |
| no-misused-promises | 132/215 (61.4%) | impl (conditionals + spreads), default-on (MVP) |
| no-unsafe-call | 23/38 (60.5%) | impl |

## 40% – 60% — stubs with significant invalid-case coverage gaps

| Rule | Score | Status |
|---|---:|---|
| prefer-destructuring | 55/92 (59.8%) | stub |
| no-unnecessary-type-parameters | 95/160 (59.4%) | stub |
| no-mixed-enums | 30/51 (58.8%) | stub |
| no-unnecessary-type-conversion | 38/66 (57.6%) | stub |
| no-unnecessary-type-assertion | 127/223 (57.0%) | stub |
| use-unknown-in-catch-callback-variable | 31/56 (55.4%) | stub |
| no-unnecessary-condition | 163/296 (55.1%) | stub |
| non-nullable-type-assertion-style | 11/20 (55.0%) | stub |
| prefer-reduce-type-parameter | 17/31 (54.8%) | stub |
| return-await | 51/95 (53.7%) | stub |
| no-unsafe-return | 33/62 (53.2%) | impl (any return) |
| no-redundant-type-constituents | 55/104 (52.9%) | stub |
| switch-exhaustiveness-check | 54/104 (51.9%) | stub |
| require-array-sort-compare | 17/33 (51.5%) | stub |
| consistent-type-exports | 24/47 (51.1%) | stub |
| strict-void-return | 105/210 (50.0%) | stub |
| restrict-plus-operands | 59/119 (49.6%) | stub |
| no-confusing-void-expression | 53/108 (49.1%) | stub |
| prefer-readonly | 79/162 (48.8%) | stub |
| prefer-string-starts-ends-with | 58/123 (47.2%) | stub |
| no-unnecessary-qualifier | 8/17 (47.1%) | stub |
| no-unsafe-type-assertion | 7/15 (46.7%) | stub |
| naming-convention | 41/88 (46.6%) | stub |
| no-unsafe-member-access | 16/35 (45.7%) | partial impl (needs debug) |
| strict-boolean-expressions | 97/214 (45.3%) | partial, default-on (MVP) |
| prefer-nullish-coalescing | 275/617 (44.6%) | stub |
| prefer-find | 19/45 (42.2%) | stub |
| no-duplicate-type-constituents | 34/82 (41.5%) | stub |
| prefer-optional-chain | 18/45 (40.0%) | stub |

## < 40% — heavy lift remaining

| Rule | Score | Status |
|---|---:|---|
| no-unsafe-assignment | 36/91 (39.6%) | partial, default-on (MVP) |
| prefer-return-this-type | 8/21 (38.1%) | stub |
| no-deprecated | 93/262 (35.5%) | stub |
| no-misused-spread | 40/128 (31.2%) | stub |
| prefer-includes | 13/42 (31.0%) | stub |

## Per-rule implementation docs

The implemented rules with notable work documented:
- `docs/TSEC-COMPAT-NO-FLOATING-PROMISES.md` — 100%, full options
- `docs/TSEC-COMPAT-NO-BASE-TO-STRING.md` — 98.7%, full options
- `docs/TSEC-COMPAT-NO-MISUSED-PROMISES.md` — 61.4%, three options
- `docs/TSEC-COMPAT-NO-UNSAFE-ASSIGNMENT.md` — 39.6%, scope notes
- `docs/TSEC-COMPAT-STRICT-BOOLEAN-EXPRESSIONS.md` — 45.3%, scope notes

## On-disk configuration

The 5 MVP rules default to `error` severity. The 56 additional rules
default to `off` — opt in via `.tsgolintrc.json`:

```json
{
  "rules": {
    "no-array-delete": "error",
    "only-throw-error": "warning",
    "await-thenable": ["error"]
  }
}
```

Rules that accept options use the typescript-eslint tuple shape:

```json
{
  "rules": {
    "no-floating-promises": ["error", {"ignoreVoid": false, "checkThenables": true}],
    "no-base-to-string": ["error", {"ignoredTypeNames": ["MyBrand"]}],
    "no-misused-promises": ["error", {"checksConditionals": true}]
  }
}
```

Unknown option keys exit with code 2 and a structured error pointing
at the offending key.

## Notes on the stub baseline

A no-op rule scores higher than zero on this metric because most
upstream cases are valid (no diagnostic expected); a no-op passes
those by definition. The implementation gap for stubs is concentrated
in the `invalid` half of each fixture. The implementations marked
above explicitly DO emit diagnostics for the invalid cases they
target.
