# typescript-eslint compatibility: strict-boolean-expressions

**97/214 cases passed (45.3%)** — baseline implementation only.

## What's implemented

- IfStatement, ConditionalExpression, WhileStatement test position
- Defaults: allow boolean, string, number; flag nullables and unknown

## What's missing (the 55% gap)

- Many more test positions: DoStatement, ForStatement, BinaryExpression
  (logical operators), PrefixUnaryExpression (`!`)
- Full option set: `allowString`, `allowNumber`, `allowBoolean`,
  `allowNullableObject`, `allowNullableBoolean`, `allowNullableString`,
  `allowNullableNumber`, `allowAny`, `allowNullableEnum`,
  `allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing`
- Per-type fine-grained handling (object types, enums, never, etc.)
- ParenthesizedExpression peeling

This is a wide-surface rule with intricate option semantics. Plumbing
all the option combinations through is the bulk of the work.

## Reproduce

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/strictbooleanexpressions/
```
