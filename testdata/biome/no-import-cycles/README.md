# no-import-cycles fixtures

Vendored verbatim from biome at
`crates/biome_js_analyze/tests/specs/suspicious/noImportCycles/`
(biome SHA `15fdfdf92e382993d7a826f50abde6220be1edd7`).

Layout matters: the rule operates on the whole program, so each `.js`
and `.ts` file's import edges must resolve to actual sibling files
inside this directory tree. The harness loads the directory through a
generated `tsconfig.json` and lints every file in one program load.

Expected diagnostic counts (per file, with the configured options
shown):

- `invalidFoobar.js` — 1, default options: cycles back through
  `invalidBaz.js`.
- `invalidBaz.js` — 1, default options: cycles back through
  `invalidFoobar.js`.
- `valid.js` — 0, default options: the import of `./invalidFoobar`
  resolves but does not close a cycle back to `valid.js`; the
  `./valid` self-imports are exempt.
- `types.ts` — 0, default options (`ignoreTypes: true`): all imports
  are `import type`, stripped before cycle detection.
- `includeTypes.ts` — 1, with `ignoreTypes: false`
  (from `includeTypes.options.json`): the `import type { Foo }`
  closes a cycle with `types.ts`.
- `ignoreTypes/{a,b,c}.ts` — 0, default options: the cycle exists
  only via type-only imports (and a non-type-only edge from `c.ts`
  back to `a.ts` which is reached only through `b.ts`'s type-only
  chain).
