# typescript-eslint compatibility: no-misused-promises

**132/215 cases passed (61.4%)** — partial implementation.

## Reproduce

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/nomisusedpromises/
```

## What's implemented

- **Conditional checks**: `if (p)`, `while (p)`, `do…while (p)`, `for (;p;)`, `p ? a : b`, `!p`, `p && x`, `p || x`, `p ?? x`
- **Void-return callback**: `arr.forEach(async () => …)` — async callback in a void-returning parameter slot
- **Promise spread**: `[...promise]` array spread
- **Options**: `checksConditionals`, `checksVoidReturn` (boolean or sub-config), `checksSpreads` — wired through `.tsgolintrc.json`

## What's not yet implemented (the 38% gap)

- **Variable assignment to void-return type**: `let f: () => void; f = async () => {…}`
- **Object literal property with contextual void-return type**: `const obj: O = { f: async () => 0 }` where `O.f: () => void`
- **JSX attribute with void-return type**: `<Component func={async () => 0} />`
- **Function return value**: `function f(): () => void { return async () => 0; }`
- **Object spread of Promise**: `{ ...promise }` (object literal spread)
- **Optional chaining call**: `returnsPromise?.()` shape
- **Nullish-coalescing edge cases**: `p ?? x` with various operand types
- **Overload resolution**: `interface ItLike { (cb: () => void): void; (cb: () => Promise<void>): void; }` — we over-flag

These require deeper contextual-type integration with the wrapper
(`GetContextualType` for assignment expressions, object literal
elements, JSX attributes, return expressions).

## Configuration

```json
{
  "rules": {
    "no-misused-promises": ["error", {
      "checksConditionals": true,
      "checksVoidReturn": true,
      "checksSpreads": true
    }]
  }
}
```
