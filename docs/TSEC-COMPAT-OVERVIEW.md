# typescript-eslint compatibility overview

Per-rule compatibility against typescript-eslint's published test
fixtures. Each rule has a vendored fixture under
`testdata/typescript-eslint/<rule>.test.ts` and a compatibility harness
under `internal/rules/<pkg>/tselintcompat_test.go`.

**Aggregate: 5975/6193 cases pass (96.5%).**

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
| prefer-string-starts-ends-with | 123/123 |
| prefer-find | 45/45 |
| promise-function-async | 53/53 |
| no-unsafe-return | 62/62 |
| await-thenable | 121/121 |
| prefer-return-this-type | 21/21 |
| no-unnecessary-template-expression | 71/71 |
| consistent-return | 30/30 |
| unbound-method | 202/202 |
| no-misused-promises | 215/215 |
| use-unknown-in-catch-callback-variable | 56/56 |
| no-unsafe-member-access | 35/35 |
| no-unsafe-assignment | 91/91 |
| prefer-promise-reject-errors | 161/161 |
| prefer-readonly | 162/162 |
| no-mixed-enums | 51/51 |
| return-await | 95/95 |
| prefer-nullish-coalescing | 617/617 |
| prefer-destructuring | 92/92 |

## ≥ 90%

| Rule | Score | Notes |
|---|---:|---|
| no-misused-spread | 127/128 (99.2%) | impl |
| dot-notation | 60/61 (98.4%) | impl |
| no-unnecessary-boolean-literal-compare | 44/45 (97.8%) | 1 case needs `strictNullChecks: false` per-case |
| no-unnecessary-qualifier | 16/17 (94.1%) | impl |
| no-deprecated | 249/262 (95.0%) | cross-module re-exports, JSX intrinsic attrs |
| no-useless-default-assignment | 79/83 (95.2%) | noStrictNullCheck variant + complex callbacks |
| switch-exhaustiveness-check | 100/104 (96.2%) | defaultCaseCommentPattern + bug fix |
| prefer-readonly-parameter-types | 119/130 (91.5%) | TypeOrValueSpecifier qualifiers + Readonly<T> wrapper |
| consistent-type-exports | 43/47 (91.5%) | export-all + namespace |
| no-unnecessary-type-assertion | 210/223 (94.2%) | flow analysis, complex contextual narrowing |
| no-unnecessary-condition | 281/296 (94.9%) | complex chain narrowing, keyof/generic widening |
| strict-boolean-expressions | 208/214 (97.2%) | spread args, complex overload signatures |
| no-unnecessary-type-parameters | 155/160 (96.9%) | inferred return type inference |

## 40 – 60%

| Rule | Score |
|---|---:|
| strict-void-return | 114/210 (54.3%) |
| naming-convention | 45/88 (51.1%) |

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
