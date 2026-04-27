# typescript-eslint compatibility: no-base-to-string

**311/315 cases passed (98.7%)** against typescript-eslint's published
test fixtures.

## Reproduce

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/nobasetotostring/
```

## What the rule recognizes

- Template-literal interpolation: `` `${x}` ``
- Explicit conversion calls: `x.toString()`, `x.toLocaleString()`, `String(x)`
- `array.join(...)` where any element lacks meaningful toString
- String concatenation: `'pre' + x` and `x += 'suffix'`
- Tagged template literals are skipped (the tag function owns the conversion)
- Promise/RegExp/Date/Symbol/Map/Set/etc. as well-known types with meaningful toString
- Class extends-chain walking for `IgnoredTypeNames` matching

## Options

| Option | Default | What it does |
|---|---|---|
| `IgnoredTypeNames` | `["Error", "RegExp", "URL", "URLSearchParams"]` | Type names whose values are not flagged. Matches by symbol, alias, or any inherited base name. |
| `CheckUnknown` | `false` | When true, `unknown` and `any` are flagged in string-conversion positions. |

Configure via `.tsgolintrc.json`:

```json
{
  "rules": {
    "no-base-to-string": ["error", {
      "ignoredTypeNames": ["Error", "RegExp", "URL", "URLSearchParams", "MyBrand"],
      "checkUnknown": true
    }]
  }
}
```

## Known limitations (4 cases)

- `Symbol.toPrimitive` method on object literals isn't detected as
  meaningful conversion (1 case)
- User-shadowed `function String(...)` still triggers when the
  shadowed function's return type happens to be string-like (1 case)
- Tuple intersections like `[string] & [Foo]` are over-conservative
  on `.join()` (1 case)
- Generic `T` with implicit `unknown` constraint isn't unwound to
  `unknown` for the type-gate (1 case)

These edge cases require deeper wrapper integration (declaration-source
detection, generic constraint unwinding) and are deferred.
