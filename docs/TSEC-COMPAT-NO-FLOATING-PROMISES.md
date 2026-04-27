# typescript-eslint compatibility: no-floating-promises

A snapshot of how our `no-floating-promises` rule scores against
typescript-eslint's official test fixtures, captured by running every
`valid`/`invalid` case from their published test file through our rule
and comparing the diagnostic count.

## Headline number

**175/175 cases passed (100.0%)**.

The rule's behavioral surface is a Go reimplementation of
typescript-eslint's `no-floating-promises` (MIT licensed). The
recursion order, type-gate placement, chain-handler semantics,
allow-list semantics, and option set are derived from upstream's
source so users get matching behavior. Code structure and the
underlying type-checker API are ours; observable behavior matches
upstream's, verified by the harness.

The vendored test suite (`testdata/typescript-eslint/`) keeps
upstream's MIT LICENSE alongside the test file.

## Reproduce

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/nofloatingpromises/
```

The harness lives at
`internal/rules/nofloatingpromises/tselintcompat_test.go` and pulls
cases via `internal/tselintcompat`, which parses the vendored
`testdata/typescript-eslint/no-floating-promises.test.ts` (5 618 lines,
upstream `main` branch) using tsgo's parser and walks the AST to
extract the `ruleTester.run(...)` argument as data — including the
`options` object for option-dependent cases.

## What the rule recognizes

Floating-promise detection covers, beyond a bare `Promise<T>` value:

- **Promise-chain handlers** — `.catch(fn)`, `.then(_, fn)`, `.then(...).finally(...)` (the `.finally` requires a handled prefix); the rejection-handler argument's type must be unambiguously callable, so `.catch(null)` / `.catch(3)` / `.then(_, definitelyMaybeString)` still flag
- **Promise subclasses** — `class MyPromise<T> extends Promise<T> {}` instances flag, including across generic instantiation
- **Intersection types** — `Promise<T> & { ... }` flags (any constituent that's a Promise marks the whole type)
- **Type aliases** — `type Foo = Promise<X> & { ... }` matches by alias name `Foo` for allow-listing
- **Array/tuple of Promises** — `Array<Promise<T>>`, `[Promise<T>, ...]`, `[..., Promise.x, ...] as const`, and union-with-such flag; covers the common `arr.map(asyncCb)` pattern
- **Generic constraints** — `function f<T extends Array<Promise<...>>>(a: T)` flags `a;` because the constraint is a Promise collection
- **Comma operator** — `(p1, x, p2);` walks every sub-expression
- **Logical/conditional operators** — both branches of `cond ? x : y`, `a || b`, `a && b`, `a ?? b` are checked, but only after the parent expression's type passes the promise-like gate (so narrowed-literal cases like `let condition = false; condition && p` don't trigger)
- **Parenthesized expressions** — peeled before any of the above
- **Spread args** — `then(...arr, fn)` is treated as not-handled because the rejection slot can't be located positionally
- **`await` of a promise array** — `await arrayOfPromises` flags because `await` doesn't unwrap inner promises

`x = somePromise` is treated as captured (not floating).

## Options

All five typescript-eslint options are implemented and behave identically:

| Option | Default | What it does |
|---|---|---|
| `IgnoreVoid` | `true` | When `true`, `void promise` is a valid suppression idiom. Set `false` to require `await` even when the value is explicitly discarded. |
| `IgnoreIIFE` | `false` | When `true`, `(async () => { ... })()` at statement position is not flagged. |
| `CheckThenables` | `false` | When `true`, any structural thenable (object with a callable `then`) is treated like a Promise. |
| `AllowForKnownSafePromises` | `[]` | Type matchers (`{ from, name }` or bare string) naming Promise types the user has marked safe. Matches by symbol name **or** type-alias name, and walks union/intersection members. |
| `AllowForKnownSafeCalls` | `[]` | Callee matchers; when a call's resolved callee matches one of these, the call is not flagged. Matches the callee's symbol name or its type's symbol name. Applied at the top of each ExpressionStatement only (not inside the recursion). |

The `from` field of a matcher (`'file' | 'lib' | 'package'`) is parsed
and stored but not currently used to disambiguate — we match by name
across all sources. This covers every case in the upstream fixture
suite. Adding source discrimination is a follow-up if a real-world
codebase needs it.

## Caveats

- The harness uses `strict: true, target: es2022, lib: [es2022, dom],
  skipLibCheck: true`, which is close to but not identical to
  typescript-eslint's fixture tsconfig (`target: es2015, types:
  [node, react]`). The compat numbers are sensitive to that — if you
  run the upstream tests locally with their tsconfig the absolute
  count may differ slightly.
- The harness writes one temp project per case (~175 program loads)
  and adds ~5 s to the rule package's test run. It's gated with
  `testing.Short()` for that reason.
- `ExpectedErrorCount` is the length of the upstream `errors:` array;
  we don't currently match on message ID or suggested fix.
- Per-rule options are now first-class in `.tsgolintrc.json` using
  the typescript-eslint tuple shape:

  ```json
  {
    "rules": {
      "no-floating-promises": ["error", {
        "ignoreVoid": false,
        "checkThenables": true,
        "allowForKnownSafePromises": [{"from": "file", "name": "Foo"}]
      }]
    }
  }
  ```

  The bare-string form (`"no-floating-promises": "error"`) still
  works and yields default options. Cascade is replace-not-merge:
  the deepest config that mentions a rule wins for both severity
  and options. Unknown option keys are rejected at config-load
  time so typos surface fast (exit code 2, structured error).
