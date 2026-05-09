# typescript-eslint compatibility overview

Per-rule compatibility against typescript-eslint's published test
fixtures. Each rule has a vendored fixture under
`testdata/typescript-eslint/<rule>.test.ts` and a compatibility harness
under `internal/rules/<pkg>/tselintcompat_test.go`.

**Aggregate: 5046/6193 cases pass (81.5%).**

Run any single rule's harness with:

```bash
go -C ~/src/lint test -count=1 -run TypescriptEslintCompatibility -v ./internal/rules/<pkg>/
```

## 100% — feature-complete against the upstream fixtures

| Rule | Score |
|---|---:|
| no-base-to-string | 315/315 |
| no-floating-promises | 175/175 |
| restrict-plus-operands | 119/119 |
| no-confusing-void-expression | 108/108 |
| no-redundant-type-constituents | 104/104 |
| only-throw-error | 89/89 |
| no-unsafe-enum-comparison | 85/85 |
| no-duplicate-type-constituents | 82/82 |
| restrict-template-expressions | 79/79 |
| no-unnecessary-type-arguments | 71/71 |
| no-implied-eval | 70/70 |
| no-unnecessary-type-conversion | 66/66 |
| require-await | 54/54 |
| prefer-optional-chain | 45/45 |
| no-unsafe-argument | 42/42 |
| prefer-includes | 42/42 |
| no-unsafe-call | 38/38 |
| prefer-regexp-exec | 37/37 |
| require-array-sort-compare | 33/33 |
| prefer-reduce-type-parameter | 31/31 |
| no-array-delete | 29/29 |
| no-unsafe-unary-minus | 23/23 |
| related-getter-setter-pairs | 23/23 |
| no-for-in-array | 22/22 |
| non-nullable-type-assertion-style | 20/20 |
| no-unsafe-type-assertion | 15/15 |
| no-meaningless-void-operator | 5/5 |

## ≥ 90%

| Rule | Score | Notes |
|---|---:|---|
| await-thenable | 120/121 (99.2%) | impl |
| prefer-promise-reject-errors | 159/161 (98.8%) | impl |
| no-unsafe-return | 61/62 (98.4%) | impl |
| no-misused-spread | 126/128 (98.4%) | impl |
| dot-notation | 60/61 (98.4%) | impl |
| use-unknown-in-catch-callback-variable | 55/56 (98.2%) | impl |
| no-mixed-enums | 50/51 (98.0%) | impl |
| no-unnecessary-boolean-literal-compare | 44/45 (97.8%) | 1 case needs `strictNullChecks: false` per-case |
| prefer-return-this-type | 20/21 (95.2%) | impl |
| prefer-readonly | 153/162 (94.4%) | impl |
| no-unsafe-member-access | 33/35 (94.3%) | impl |

## 70 – 90%

| Rule | Score | Status |
|---|---:|---|
| consistent-return | 26/30 (86.7%) | impl |
| no-unnecessary-template-expression | 61/71 (85.9%) | impl |
| prefer-readonly-parameter-types | 106/130 (81.5%) | impl |
| strict-boolean-expressions | 172/214 (80.4%) | impl |
| prefer-destructuring | 72/92 (78.3%) | impl |
| switch-exhaustiveness-check | 81/104 (77.9%) | impl |
| promise-function-async | 41/53 (77.4%) | impl |
| no-unsafe-assignment | 70/91 (76.9%) | impl |
| no-misused-promises | 165/215 (76.7%) | partial impl, default-on |
| no-deprecated | 201/262 (76.7%) | impl, walks alias chain & ElementAccess |
| no-unnecessary-qualifier | 13/17 (76.5%) | impl |
| no-unnecessary-condition | 226/296 (76.4%) | impl |
| no-useless-default-assignment | 60/83 (72.3%) | stub baseline |
| prefer-find | 32/45 (71.1%) | impl |

## 60 – 70%

| Rule | Score | Status |
|---|---:|---|
| unbound-method | 141/202 (69.8%) | stub baseline |
| no-unnecessary-type-assertion | 152/223 (68.2%) | impl |
| return-await | 63/95 (66.3%) | stub baseline |
| prefer-string-starts-ends-with | 80/123 (65.0%) | stub baseline |

## 40 – 60%

| Rule | Score |
|---|---:|
| no-unnecessary-type-parameters | 95/160 (59.4%) |
| strict-void-return | 114/210 (54.3%) |
| naming-convention | 45/88 (51.1%) |
| consistent-type-exports | 24/47 (51.1%) |
| prefer-nullish-coalescing | 303/617 (49.1%) |

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
