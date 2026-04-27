# typescript-eslint compatibility overview

Per-rule compatibility against typescript-eslint's published test
fixtures. Each rule has a vendored fixture under
`testdata/typescript-eslint/<rule>.test.ts` and a compatibility harness
under `internal/rules/<pkg>/tselintcompat_test.go`.

Run any single rule's harness with:

```bash
go -C ~/src/lint test -count=1 -run TypescriptEslintCompatibility -v ./internal/rules/<pkg>/
```

## 100% — feature-complete against the upstream fixtures

| Rule | Score |
|---|---:|
| no-floating-promises | 175/175 |
| no-base-to-string | 315/315 |
| only-throw-error | 89/89 |
| no-implied-eval | 70/70 |
| no-array-delete | 29/29 |
| no-unsafe-unary-minus | 23/23 |
| no-for-in-array | 22/22 |
| no-meaningless-void-operator | 5/5 |

## ≥ 90%

| Rule | Score | Notes |
|---|---:|---|
| no-unnecessary-boolean-literal-compare | 44/45 (97.8%) | 1 case needs a per-case tsconfig with `strictNullChecks: false`; harness always writes strict |
| no-unsafe-argument | 41/42 (97.6%) | 1 case is generic deep-any positional comparison |

## 70 – 90%

| Rule | Score | Status |
|---|---:|---|
| restrict-template-expressions | 69/79 (87.3%) | impl with all options |
| no-unsafe-enum-comparison | 73/85 (85.9%) | impl |
| no-unnecessary-template-expression | 56/71 (78.9%) | stub baseline (not implemented) |
| await-thenable | 89/121 (73.6%) | impl |
| no-useless-default-assignment | 60/83 (72.3%) | stub baseline |

## 60 – 70%

| Rule | Score | Status |
|---|---:|---|
| unbound-method | 141/202 (69.8%) | stub baseline |
| related-getter-setter-pairs | 16/23 (69.6%) | stub baseline |
| prefer-promise-reject-errors | 112/161 (69.6%) | impl (partial) |
| require-await | 37/54 (68.5%) | stub baseline |
| prefer-readonly-parameter-types | 89/130 (68.5%) | stub baseline |
| prefer-regexp-exec | 25/37 (67.6%) | stub baseline |
| no-unnecessary-type-arguments | 48/71 (67.6%) | stub baseline |
| consistent-return | 19/30 (63.3%) | stub baseline |
| promise-function-async | 33/53 (62.3%) | impl |
| dot-notation | 38/61 (62.3%) | stub baseline |
| no-misused-promises | 132/215 (61.4%) | partial impl, default-on (MVP) |
| no-unsafe-call | 23/38 (60.5%) | impl |

## 40 – 60%

(Most rules are stubs with high baselines because most upstream cases
are valid; the implementation gap is concentrated in the `invalid` half
of each fixture.)

| Rule | Score |
|---|---:|
| prefer-destructuring | 55/92 (59.8%) |
| no-unnecessary-type-parameters | 95/160 (59.4%) |
| no-mixed-enums | 30/51 (58.8%) |
| no-unnecessary-type-conversion | 38/66 (57.6%) |
| no-unnecessary-type-assertion | 127/223 (57.0%) |
| use-unknown-in-catch-callback-variable | 31/56 (55.4%) |
| no-unnecessary-condition | 163/296 (55.1%) |
| non-nullable-type-assertion-style | 11/20 (55.0%) |
| prefer-reduce-type-parameter | 17/31 (54.8%) |
| return-await | 51/95 (53.7%) |
| no-unsafe-return | 33/62 (53.2%) |
| no-redundant-type-constituents | 55/104 (52.9%) |
| switch-exhaustiveness-check | 54/104 (51.9%) |
| require-array-sort-compare | 17/33 (51.5%) |
| consistent-type-exports | 24/47 (51.1%) |
| strict-void-return | 105/210 (50.0%) |
| restrict-plus-operands | 59/119 (49.6%) |
| no-confusing-void-expression | 53/108 (49.1%) |
| prefer-readonly | 79/162 (48.8%) |
| prefer-string-starts-ends-with | 58/123 (47.2%) |
| no-unnecessary-qualifier | 8/17 (47.1%) |
| no-unsafe-type-assertion | 7/15 (46.7%) |
| naming-convention | 41/88 (46.6%) |
| no-unsafe-member-access | 16/35 (45.7%) |
| strict-boolean-expressions | 97/214 (45.3%) |
| prefer-nullish-coalescing | 275/617 (44.6%) |
| prefer-find | 19/45 (42.2%) |
| no-duplicate-type-constituents | 34/82 (41.5%) |
| prefer-optional-chain | 18/45 (40.0%) |

## < 40%

| Rule | Score |
|---|---:|
| no-unsafe-assignment | 36/91 (39.6%) |
| prefer-return-this-type | 8/21 (38.1%) |
| no-deprecated | 93/262 (35.5%) |
| no-misused-spread | 40/128 (31.2%) |
| prefer-includes | 13/42 (31.0%) |

## Per-rule docs

- `docs/TSEC-COMPAT-NO-FLOATING-PROMISES.md`
- `docs/TSEC-COMPAT-NO-BASE-TO-STRING.md`
- `docs/TSEC-COMPAT-NO-MISUSED-PROMISES.md`
- `docs/TSEC-COMPAT-NO-UNSAFE-ASSIGNMENT.md`
- `docs/TSEC-COMPAT-STRICT-BOOLEAN-EXPRESSIONS.md`

## On-disk configuration

The 5 MVP rules default to `error` severity. The 56 additional rules
default to `off` — opt in via `.tsgolintrc.json`:

```json
{
  "rules": {
    "no-array-delete": "error",
    "only-throw-error": ["error", { "allowThrowingAny": false }]
  }
}
```

Unknown option keys exit with code 2 and a structured error.
