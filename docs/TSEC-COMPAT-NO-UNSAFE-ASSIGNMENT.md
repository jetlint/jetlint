# typescript-eslint compatibility: no-unsafe-assignment

**36/91 cases passed (39.6%)** — baseline implementation only.

## What's implemented

- Variable declarations where the initializer's type is `any` and the
  declared LHS type is more specific.

## What's missing (scope for the next pass)

- `as any` casts (the explicit-narrowing case): `const x = 1 as any`
- Assignment expressions: `value = spookyAny`
- Parameter default values: `function foo(a = 1 as any)`
- Class property initializers: `class Foo { a = 1 as any; }`
- Destructuring: `const [x] = spooky`, `const {x} = spooky`
- Generic type-argument mismatches: `Set<string> = new Set<any>()` (deep)

This rule needs an extensive rewrite to track every assignment site
(VariableDeclaration, BinaryExpression with `=`, ParameterDeclaration,
PropertyDeclaration, BindingElement) and to compare type structurally
across generic parameters.

## Reproduce

```bash
go -C ~/src/lint test -run TypescriptEslintCompatibility -v ./internal/rules/nounsafeassignment/
```
