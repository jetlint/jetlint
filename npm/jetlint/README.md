# jetlint

Fast, type-aware TypeScript 7 linter. Drop-in compatible with typescript-eslint.

```bash
npm install --save-dev jetlint
npx jetlint --project ./tsconfig.json
```

This package is a thin Node.js wrapper that selects the correct prebuilt binary
for your platform via npm's `optionalDependencies` mechanism. The binaries
themselves live in the per-platform packages:

- `@jetlint/linux-x64`
- `@jetlint/linux-arm64`
- `@jetlint/darwin-x64`
- `@jetlint/darwin-arm64`
- `@jetlint/win32-x64`

If your platform isn't listed, install Go 1.26+ and run
`go install github.com/jetlint/jetlint/cmd/jetlint@latest`.

**Site & docs:** https://jetlint.github.io
**Source:** https://github.com/jetlint/jetlint

Licensed under MIT.
