// Package useexhaustivedependencies implements use-exhaustive-dependencies:
// React reactive hooks (useEffect, useCallback, useMemo,
// useImperativeHandle, useLayoutEffect, useInsertionEffect) accept a
// dependency array as their second argument. Any value referenced in
// the callback body that comes from an enclosing scope must appear in
// that array, or React may skip re-running the effect when it should.
// The rule walks each call's callback body collecting referenced free
// identifiers, then verifies each one appears in the dependency
// array. Values declared inside the callback are excluded. The rule
// is intentionally conservative: it does not yet handle property
// access dependencies (`obj.foo`) or recognize stable setters from
// useState/useReducer.
package useexhaustivedependencies

import (
	"os"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-exhaustive-dependencies"

// reactiveHooks is the set of React hooks whose second argument is a
// dependency array. Matched by callee identifier or property-access
// name; namespace prefixes (`React.useEffect`) are accepted.
var reactiveHooks = map[string]bool{
	"useEffect":            true,
	"useLayoutEffect":      true,
	"useInsertionEffect":   true,
	"useCallback":          true,
	"useMemo":              true,
	"useImperativeHandle":  true,
}

// globallyAvailable names identifiers that don't need to be declared
// as deps because they are not reactive (built-ins, globals,
// commonly-imported singletons).
var globallyAvailable = map[string]bool{
	"undefined": true, "null": true, "true": true, "false": true,
	"Math": true, "Date": true, "JSON": true, "Object": true, "Array": true,
	"String": true, "Number": true, "Boolean": true, "Symbol": true,
	"Promise": true, "Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
	"RegExp": true, "Error": true, "TypeError": true, "RangeError": true,
	"console": true, "window": true, "document": true, "globalThis": true,
	"setTimeout": true, "setInterval": true, "clearTimeout": true, "clearInterval": true,
	"requestAnimationFrame": true, "cancelAnimationFrame": true,
	"fetch": true, "URL": true, "URLSearchParams": true,
	"parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true,
	"this": true, "arguments": true, "super": true,
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCall,
	}
}

func visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	if !isReactiveHook(call) {
		return
	}
	if !hookIsFromReact(call) {
		return
	}
	callback, deps, depsNode := hookCallbackAndDeps(call)
	if callback == nil {
		return
	}
	suppress := suppressedNames(call)
	if suppress.all {
		return
	}
	// `deps` is the actual ArrayLiteralExpression. `depsNode` is the
	// second argument as written — a non-array (e.g., a variable
	// reference like `useEffect(fn, depsVar)`) means the rule can't
	// verify anything; flag the spelling itself.
	if deps == nil {
		if depsNode != nil {
			ctx.Report(depsNode, "Hook dependencies must be an array literal so the rule can verify them.")
		}
		return
	}
	moduleScope := moduleScopeNames(call)
	stableSetters := stableSetterNames(call)
	literalConsts := literalConstNames(call)
	declaredEntries := depEntries(deps)
	declared := map[string]bool{}
	pathCounts := map[string]int{}
	for _, e := range declaredEntries {
		declared[e.name] = true
		pathCounts[e.path]++
	}
	// Duplicate entries — two array elements with the *same* path
	// (`[a, a]` or `[obj.x, obj.x]`) — are a hard error: React
	// diffs the array by index, so a duplicate either masks a real
	// change or is dead weight. Distinct property paths sharing a
	// head (`[obj.x, obj.y]`) are not duplicates.
	for _, e := range declaredEntries {
		if pathCounts[e.path] > 1 {
			ctx.Report(e.node, "This dependency is listed more than once.")
			pathCounts[e.path] = 0
		}
	}
	used := map[string]bool{}
	bodyRefs := freeIdentifiers(callback)
	for _, ref := range bodyRefs {
		name := ref.LiteralText()
		if name == "" || globallyAvailable[name] {
			continue
		}
		// Module-level bindings (imports, top-level const/let/var,
		// function declarations) don't change across renders, so
		// they don't need to be listed as dependencies.
		if moduleScope[name] {
			used[name] = true
			continue
		}
		// Setters returned from useState/useReducer have stable
		// identity guaranteed by React; listing them is allowed
		// but not required.
		if stableSetters[name] {
			used[name] = true
			continue
		}
		// A `const X = <literal>` inside the component body is a
		// compile-time constant — its value is the same across
		// every render, so React never has to re-run on its
		// account.
		if literalConsts[name] {
			used[name] = true
			continue
		}
		if declared[name] {
			used[name] = true
			continue
		}
		// A `// biome-ignore lint/correctness/useExhaustiveDependencies(name)`
		// directive on the call's leading line suppresses the
		// missing-dep diagnostic for that name. We still mark it
		// "used" so the extra-dep loop later sees it as accounted
		// for if it appears in the array.
		if suppress.names[name] {
			used[name] = true
			continue
		}
		ctx.Report(ref, "This dependency is not specified in the hook dependency list.")
	}
	// Extra dependencies — listed in the array but never read inside
	// the callback. React still re-runs on their changes, so they
	// can cause unnecessary work.
	for _, e := range declaredEntries {
		if suppress.names[e.name] {
			continue
		}
		if !used[e.name] && !globallyAvailable[e.name] {
			ctx.Report(e.node, "This dependency is not used in the hook callback.")
		}
	}
	// Unstable dependencies — locally-declared values (arrow
	// functions, function declarations, object/array literals) are
	// recreated each render. Listing them in deps defeats the
	// effect's purpose: it would re-run on every render.
	unstable := unstableLocalNames(call)
	for _, e := range declaredEntries {
		if suppress.names[e.name] {
			continue
		}
		if unstable[e.name] {
			ctx.Report(e.node, e.name+" changes on every re-render and should not be used as a hook dependency.")
		}
	}
}

// suppression captures the biome-ignore directives that immediately
// precede a hook call. `all` is true for a bare
// `// biome-ignore lint/correctness/useExhaustiveDependencies`
// without a parenthesized name list; `names` holds explicit names
// from `(...)` forms.
type suppression struct {
	all   bool
	names map[string]bool
}

// suppressedNames scans the source lines immediately preceding the
// call's statement for biome-ignore directives addressing this rule.
// The lookup uses 1-based line numbers from SourceRange because the
// AST Pos() value covers leading trivia, which makes naive
// substring-of-prefix scanning ambiguous.
func suppressedNames(call *wrapperchecker.Node) suppression {
	res := suppression{names: map[string]bool{}}
	stmt := containingStatement(call)
	if stmt == nil {
		stmt = call
	}
	filePath, startLine, _, _, _ := stmt.SourceRange()
	if startLine <= 0 || filePath == "" {
		return res
	}
	// SourceText() on a SourceFile strips leading file-level trivia,
	// so its line numbers don't line up with the original file the
	// way SourceRange's do. Read the actual file off disk instead.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return res
	}
	lines := strings.Split(string(data), "\n")
	// SourceRange's line numbering is 1-based; the source array is
	// 0-indexed, so the line immediately preceding the statement is
	// at index startLine - 2.
	for i := startLine - 2; i >= 0 && i < len(lines); i-- {
		line := lines[i]
		if all, names, ok := parseBiomeIgnore(line); ok {
			if all {
				res.all = true
			}
			for _, n := range names {
				res.names[n] = true
			}
			continue
		}
		if strings.TrimSpace(line) == "" || !isCommentLine(line) {
			break
		}
	}
	return res
}

func isCommentLine(line string) bool {
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/*") || strings.HasPrefix(s, "*")
}

// parseBiomeIgnore matches `// biome-ignore lint/correctness/useExhaustiveDependencies`
// or its `(name1, name2)` form. Returns (all, names, ok).
func parseBiomeIgnore(line string) (bool, []string, bool) {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "//")
	s = strings.TrimPrefix(s, "/*")
	s = strings.TrimSpace(s)
	const prefix = "biome-ignore"
	if !strings.HasPrefix(s, prefix) {
		return false, nil, false
	}
	s = strings.TrimSpace(s[len(prefix):])
	const rulePath = "lint/correctness/useExhaustiveDependencies"
	if !strings.HasPrefix(s, rulePath) {
		return false, nil, false
	}
	s = s[len(rulePath):]
	if strings.HasPrefix(s, "(") {
		end := strings.Index(s, ")")
		if end <= 0 {
			return false, nil, true
		}
		inner := s[1:end]
		var names []string
		for _, part := range strings.Split(inner, ",") {
			if t := strings.TrimSpace(part); t != "" {
				names = append(names, t)
			}
		}
		return false, names, true
	}
	return true, nil, true
}

// containingStatement returns the nearest ancestor that is a
// statement (the level where a leading-line comment would attach).
func containingStatement(n *wrapperchecker.Node) *wrapperchecker.Node {
	cur := n.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindExpressionStatement,
			wrapperchecker.KindVariableStatement,
			wrapperchecker.KindReturnStatement,
			wrapperchecker.KindIfStatement,
			wrapperchecker.KindForStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindBlock:
			return cur
		}
		cur = cur.Parent()
	}
	return nil
}

// hookIsFromReact reports whether the hook callee comes from the
// React module — either an identifier imported from "react" / "preact"
// (any react-like alias) or a property access on a React-named
// import (`React.useEffect`, `R.useEffect` if `R` was imported from
// react). When the hook isn't recognizable as a React import,
// biome (and we) skip the check entirely because it could be a
// user-defined function that happens to share a hook's name.
func hookIsFromReact(call *wrapperchecker.Node) bool {
	callee := call.CalleeExpression()
	if callee == nil {
		return false
	}
	imports := reactImports(call)
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		if imports == nil {
			return false
		}
		return imports.names[callee.LiteralText()]
	case wrapperchecker.KindPropertyAccessExpression:
		var head *wrapperchecker.Node
		callee.ForEachChild(func(c *wrapperchecker.Node) bool {
			if head == nil {
				head = c
				return true
			}
			return false
		})
		if head == nil || head.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		name := head.LiteralText()
		if imports != nil && imports.namespaces[name] {
			return true
		}
		// `React.useX` is the canonical pre-modules pattern: even
		// without an explicit import, callers expect it to mean the
		// global React unless the surrounding code shadows it with
		// a local binding.
		if name == "React" && !nameIsLocallyBound(head, "React") {
			return true
		}
		return false
	}
	return false
}

// nameIsLocallyBound reports whether `name` is declared somewhere
// inside an enclosing function body of n (component, hook, plain
// function — anything function-shaped). Used to spot
// `const React = { ... }` shadowing the global, which makes a
// `React.useFoo` call refer to user code rather than React.
func nameIsLocallyBound(n *wrapperchecker.Node, name string) bool {
	host := n.Parent()
	for host != nil {
		switch host.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration:
			body := functionBody(host)
			if body != nil && hasLocalBinding(body, name) {
				return true
			}
		case wrapperchecker.KindSourceFile:
			return hasLocalBinding(host, name)
		}
		host = host.Parent()
	}
	return false
}

// hasLocalBinding scans body for any top-level (within body)
// variable, function, class, or import declaration introducing
// `name`. Doesn't descend through nested functions.
func hasLocalBinding(body *wrapperchecker.Node, name string) bool {
	found := false
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindVariableStatement:
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() != wrapperchecker.KindVariableDeclarationList {
					return false
				}
				g.ForEachChild(func(d *wrapperchecker.Node) bool {
					if d.Kind() != wrapperchecker.KindVariableDeclaration {
						return false
					}
					for _, n := range bindingIdentifiers(d) {
						if n == name {
							found = true
							return true
						}
					}
					return false
				})
				return found
			})
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration:
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindIdentifier && g.LiteralText() == name {
					found = true
					return true
				}
				return false
			})
		}
		return found
	})
	return found
}

type reactImportInfo struct {
	hasReactImport bool
	names          map[string]bool
	namespaces     map[string]bool
	// aliasToOriginal maps the locally-bound name back to the
	// original react export. `import { useRef as uR }` becomes
	// `uR -> useRef`. Identity mappings are present too so callers
	// can do a single lookup.
	aliasToOriginal map[string]string
}

// reactImports collects the named bindings imported from "react"
// (and `preact/compat`) in the source file containing n. Always
// returns a non-nil info — the caller decides what to do when
// hasReactImport is false.
func reactImports(n *wrapperchecker.Node) *reactImportInfo {
	root := n
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return nil
	}
	info := &reactImportInfo{
		names:           map[string]bool{},
		namespaces:      map[string]bool{},
		aliasToOriginal: map[string]string{},
	}
	root.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindImportDeclaration {
			return false
		}
		spec := stmt.ModuleSpecifier()
		if spec == nil {
			return false
		}
		text := strings.Trim(spec.LiteralText(), "\"'`")
		if !isReactLikeSpecifier(text) {
			return false
		}
		info.hasReactImport = true
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() != wrapperchecker.KindImportClause {
				return false
			}
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				switch g.Kind() {
				case wrapperchecker.KindIdentifier:
					// Default import: `import React from "react"`.
					info.namespaces[g.LiteralText()] = true
				case wrapperchecker.KindNamespaceImport:
					g.ForEachChild(func(i *wrapperchecker.Node) bool {
						if i.Kind() == wrapperchecker.KindIdentifier {
							info.namespaces[i.LiteralText()] = true
							return true
						}
						return false
					})
				case wrapperchecker.KindNamedImports:
					g.ForEachChild(func(spec *wrapperchecker.Node) bool {
						var idents []string
						spec.ForEachChild(func(i *wrapperchecker.Node) bool {
							if i.Kind() == wrapperchecker.KindIdentifier {
								idents = append(idents, i.LiteralText())
							}
							return false
						})
						if len(idents) == 0 {
							return false
						}
						local := idents[len(idents)-1]
						original := idents[0]
						info.names[local] = true
						info.aliasToOriginal[local] = original
						return false
					})
				}
				return false
			})
			return true
		})
		return false
	})
	return info
}

func isReactLikeSpecifier(s string) bool {
	switch s {
	case "react", "preact", "preact/compat", "preact/hooks":
		return true
	}
	return false
}

type depEntry struct {
	node *wrapperchecker.Node
	name string // head identifier (`obj` for `obj.x.y`)
	path string // textual normalized path (`obj.x.y`) — used to dedupe
}

// depEntries flattens the deps array into a list of (name, anchor)
// pairs, one per dependency element. Non-identifier dependencies
// (object expressions, calls) and unrecognized shapes contribute
// nothing. propertyAccessHead handles the chain-walk plus TypeScript
// wrappers, so this only needs to dispatch by top-level kind. The
// path string is the element's source text with whitespace stripped
// so two textually-equal accesses dedupe even when the AST node
// instances differ.
func depEntries(deps *wrapperchecker.Node) []depEntry {
	var out []depEntry
	deps.ForEachChild(func(c *wrapperchecker.Node) bool {
		if name := propertyAccessHead(c); name != "" {
			out = append(out, depEntry{node: c, name: name, path: normalizePath(c.SourceText())})
		}
		return false
	})
	return out
}

// normalizePath collapses internal whitespace from a dependency
// expression's source text so `obj.x` and `obj . x` compare equal.
func normalizePath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isTypeNode reports whether a node's kind represents a TypeScript
// type position (so it should be skipped when peeling expression
// wrappers like `as T`).
func isTypeNode(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindTypeReference,
		wrapperchecker.KindTypeLiteral,
		wrapperchecker.KindUnionType,
		wrapperchecker.KindIntersectionType,
		wrapperchecker.KindArrayType,
		wrapperchecker.KindTupleType,
		wrapperchecker.KindFunctionType,
		wrapperchecker.KindParenthesizedType,
		wrapperchecker.KindLiteralType,
		wrapperchecker.KindMappedType,
		wrapperchecker.KindConditionalType,
		wrapperchecker.KindIndexedAccessType,
		wrapperchecker.KindTypeOperator,
		wrapperchecker.KindRestType,
		wrapperchecker.KindOptionalType,
		wrapperchecker.KindThisType,
		wrapperchecker.KindInferType,
		wrapperchecker.KindImportType,
		wrapperchecker.KindNamedTupleMember:
		return true
	}
	return false
}

// isTypeQuery reports whether n is a `typeof X` query (which can
// appear in expression-like positions inside types but is a TS-only
// reflection that doesn't bind a runtime value).
func isTypeQuery(n *wrapperchecker.Node) bool {
	return n.Kind() == wrapperchecker.KindTypeQuery
}

// moduleScopeNames returns the set of identifiers declared at the
// top level of the source file containing n. These bindings are
// stable across renders and never need to appear in a deps array.
func moduleScopeNames(n *wrapperchecker.Node) map[string]bool {
	root := n
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return nil
	}
	out := map[string]bool{}
	root.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		switch stmt.Kind() {
		case wrapperchecker.KindImportDeclaration:
			collectImportClauseNames(stmt, out)
		case wrapperchecker.KindImportEqualsDeclaration:
			stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
				if c.Kind() == wrapperchecker.KindIdentifier {
					out[c.LiteralText()] = true
					return true
				}
				return false
			})
		case wrapperchecker.KindVariableStatement:
			collectVariableStatementNames(stmt, out)
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindEnumDeclaration,
			wrapperchecker.KindInterfaceDeclaration,
			wrapperchecker.KindTypeAliasDeclaration,
			wrapperchecker.KindModuleDeclaration:
			stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
				if c.Kind() == wrapperchecker.KindIdentifier {
					out[c.LiteralText()] = true
					return true
				}
				return false
			})
		}
		return false
	})
	return out
}

func collectImportClauseNames(imp *wrapperchecker.Node, out map[string]bool) {
	imp.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindImportClause {
			return false
		}
		c.ForEachChild(func(g *wrapperchecker.Node) bool {
			switch g.Kind() {
			case wrapperchecker.KindIdentifier:
				out[g.LiteralText()] = true
			case wrapperchecker.KindNamedImports:
				g.ForEachChild(func(spec *wrapperchecker.Node) bool {
					// Local name is the LAST identifier (handles `a as b`).
					var last *wrapperchecker.Node
					spec.ForEachChild(func(i *wrapperchecker.Node) bool {
						if i.Kind() == wrapperchecker.KindIdentifier {
							last = i
						}
						return false
					})
					if last != nil {
						out[last.LiteralText()] = true
					}
					return false
				})
			case wrapperchecker.KindNamespaceImport:
				g.ForEachChild(func(i *wrapperchecker.Node) bool {
					if i.Kind() == wrapperchecker.KindIdentifier {
						out[i.LiteralText()] = true
						return true
					}
					return false
				})
			}
			return false
		})
		return true
	})
}

func collectVariableStatementNames(stmt *wrapperchecker.Node, out map[string]bool) {
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindVariableDeclarationList {
			return false
		}
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if d.Kind() == wrapperchecker.KindVariableDeclaration {
				for _, name := range bindingIdentifiers(d) {
					out[name] = true
				}
			}
			return false
		})
		return false
	})
}

// reassignedNames returns the set of identifier names that appear on
// the left side of an assignment somewhere in the given function
// body. Stable hooks lose their stability guarantee once their
// returned binding is reassigned, so the caller subtracts this set
// from the candidate stable names.
func reassignedNames(body *wrapperchecker.Node) map[string]bool {
	out := map[string]bool{}
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindBinaryExpression && n.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
			// Left-hand side identifier is the assignment target.
			var left *wrapperchecker.Node
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if left == nil {
					left = c
					return true
				}
				return false
			})
			if left != nil && left.Kind() == wrapperchecker.KindIdentifier {
				out[left.LiteralText()] = true
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return out
}

// stableSetterNames returns the names of bindings produced inside
// the enclosing component / hook function that React guarantees to
// have stable identity across renders. These don't need to appear
// in a deps array. Recognized patterns:
//
//	const [_, setX]            = useState(...)
//	const [_, dispatch]        = useReducer(...)
//	const [_, startTransition] = useTransition(...)
//	const ref                  = useRef(...)
//	const event                = useEffectEvent(...)
//
// Property-access callees (`React.useState`) are unwrapped so the
// match is on the trailing name.
func stableSetterNames(call *wrapperchecker.Node) map[string]bool {
	host := enclosingFunctionLike(call)
	if host == nil {
		return nil
	}
	body := functionBody(host)
	if body == nil {
		return nil
	}
	out := map[string]bool{}
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindVariableDeclaration {
			recordStableBinding(n, out)
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	for name := range reassignedNames(body) {
		delete(out, name)
	}
	return out
}

// stableTupleHooks names hooks whose return is a 2-tuple of
// [value, stableSecond]: useState's setter, useReducer's dispatch,
// useTransition's startTransition. The second element is stable.
var stableTupleHooks = map[string]bool{
	"useState":     true,
	"useReducer":   true,
	"useTransition": true,
}

// stableSingleHooks names hooks whose plain return value is
// guaranteed stable: useRef returns a ref object whose identity
// doesn't change, useEffectEvent / useEvent returns a stable
// callback wrapper.
var stableSingleHooks = map[string]bool{
	"useRef":         true,
	"useEffectEvent": true,
	"useEvent":       true,
}

func recordStableBinding(decl *wrapperchecker.Node, out map[string]bool) {
	pattern, initializer := splitVariableDeclaration(decl)
	if pattern == nil || initializer == nil {
		return
	}
	init := unwrapInitializer(initializer)
	if init.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	calleeName := hookCalleeName(init.CalleeExpression())
	if calleeName == "" {
		return
	}
	// Resolve aliases — `import { useRef as uR }` then
	// `const ref = uR()` should still be recognized as a stable
	// useRef return.
	if info := reactImports(decl); info != nil {
		if orig, ok := info.aliasToOriginal[calleeName]; ok {
			calleeName = orig
		}
	}
	if stableSingleHooks[calleeName] {
		if pattern.Kind() == wrapperchecker.KindIdentifier {
			out[pattern.LiteralText()] = true
		}
		return
	}
	if stableTupleHooks[calleeName] {
		if pattern.Kind() != wrapperchecker.KindArrayBindingPattern {
			return
		}
		var i int
		pattern.ForEachChild(func(elt *wrapperchecker.Node) bool {
			if elt.Kind() == wrapperchecker.KindBindingElement {
				if i == 1 {
					name := elt.BindingElementName()
					if name != nil && name.Kind() == wrapperchecker.KindIdentifier {
						out[name.LiteralText()] = true
					}
				}
				i++
			}
			return false
		})
	}
}

// splitVariableDeclaration returns the (pattern, initializer) pair
// of a VariableDeclaration. The pattern is the binding-name slot
// (identifier or destructure pattern); the initializer is the
// post-`=` expression.
func splitVariableDeclaration(decl *wrapperchecker.Node) (*wrapperchecker.Node, *wrapperchecker.Node) {
	var pattern *wrapperchecker.Node
	var initializer *wrapperchecker.Node
	seenName := false
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName {
			switch c.Kind() {
			case wrapperchecker.KindArrayBindingPattern,
				wrapperchecker.KindObjectBindingPattern,
				wrapperchecker.KindIdentifier:
				pattern = c
				seenName = true
				return false
			}
			return false
		}
		if c.Kind() == wrapperchecker.KindEqualsToken {
			return false
		}
		if initializer == nil && c.Kind() != wrapperchecker.KindArrayBindingPattern &&
			c.Kind() != wrapperchecker.KindObjectBindingPattern {
			initializer = c
			return true
		}
		return false
	})
	return pattern, initializer
}

// hookCalleeName returns the unqualified name of a hook callee,
// stripping namespace prefixes so `React.useState` → "useState".
func hookCalleeName(callee *wrapperchecker.Node) string {
	if callee == nil {
		return ""
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return callee.LiteralText()
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName()
	}
	return ""
}

// unstableLocalNames returns the set of locally-declared bindings
// whose value is recreated each render: arrow functions, function
// expressions, function declarations, object literals, array
// literals, regex literals, and class expressions. Listing one of
// these in a deps array defeats the hook — every render produces a
// fresh reference that won't be `===` to last render's.
//
// useCallback / useMemo / useRef / useState / useReducer results are
// the standard way to opt out of this — they're stable by design and
// not flagged here.
func unstableLocalNames(call *wrapperchecker.Node) map[string]bool {
	host := enclosingFunctionLike(call)
	if host == nil {
		return nil
	}
	body := functionBody(host)
	if body == nil {
		return nil
	}
	out := map[string]bool{}
	body.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		switch stmt.Kind() {
		case wrapperchecker.KindVariableStatement:
			stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
				if c.Kind() != wrapperchecker.KindVariableDeclarationList {
					return false
				}
				c.ForEachChild(func(d *wrapperchecker.Node) bool {
					if d.Kind() != wrapperchecker.KindVariableDeclaration {
						return false
					}
					pattern, initializer := splitVariableDeclaration(d)
					if pattern == nil || initializer == nil {
						return false
					}
					if pattern.Kind() != wrapperchecker.KindIdentifier {
						return false
					}
					if isUnstableExpression(initializer) {
						out[pattern.LiteralText()] = true
					}
					return false
				})
				return false
			})
		case wrapperchecker.KindFunctionDeclaration:
			stmt.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindIdentifier {
					out[g.LiteralText()] = true
					return true
				}
				return false
			})
		}
		return false
	})
	return out
}

// isUnstableExpression reports whether n produces a new reference on
// every evaluation. Calls to useCallback/useMemo etc. are explicitly
// excluded — those are the stable counterparts.
func isUnstableExpression(n *wrapperchecker.Node) bool {
	n = unwrap(n)
	switch n.Kind() {
	case wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindNewExpression:
		return true
	case wrapperchecker.KindCallExpression:
		// Stable-returning hooks (useCallback, useMemo, useRef,
		// useState, useReducer, useTransition, useEffectEvent)
		// produce fresh values intentionally bound to a stable
		// React-managed slot — not unstable.
		callee := n.CalleeExpression()
		name := hookCalleeName(callee)
		switch name {
		case "useCallback", "useMemo", "useRef", "useState",
			"useReducer", "useTransition", "useEffectEvent",
			"useSyncExternalStore", "useContext", "useId",
			"useDeferredValue":
			return false
		}
		// Any other call's return isn't intrinsically unstable
		// (could be a stable getter); be conservative.
		return false
	}
	return false
}

// literalConstNames returns the names of `const X = <stable-expr>`
// bindings in the enclosing component / hook body. "Stable" here
// means the expression either:
//   - is a primitive literal (`const X = 1`)
//   - reads (possibly via property access) from a module-scope
//     identifier (`const X = globalConfig.debug`)
// Either way the binding's value is fixed for the lifetime of the
// render and doesn't need to be in a deps array.
func literalConstNames(call *wrapperchecker.Node) map[string]bool {
	host := enclosingFunctionLike(call)
	if host == nil {
		return nil
	}
	body := functionBody(host)
	if body == nil {
		return nil
	}
	moduleScope := moduleScopeNames(call)
	out := map[string]bool{}
	body.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindVariableStatement {
			return false
		}
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() != wrapperchecker.KindVariableDeclarationList {
				return false
			}
			if !c.IsConstVariableDeclaration() {
				return false
			}
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() != wrapperchecker.KindVariableDeclaration {
					return false
				}
				pattern, initializer := splitVariableDeclaration(d)
				if pattern == nil || initializer == nil {
					return false
				}
				if pattern.Kind() != wrapperchecker.KindIdentifier {
					return false
				}
				if isStableInitializer(initializer, moduleScope) {
					out[pattern.LiteralText()] = true
				}
				return false
			})
			return false
		})
		return false
	})
	return out
}

// isStableInitializer reports whether n is an expression whose
// value won't change render-to-render: a primitive literal, an
// identifier resolving to a module-scope binding, or a chain of
// property/element accesses rooted in one.
func isStableInitializer(n *wrapperchecker.Node, moduleScope map[string]bool) bool {
	if isLiteralExpression(n) {
		return true
	}
	head := propertyAccessHead(n)
	if head == "" {
		return false
	}
	return moduleScope[head]
}

// isLiteralExpression reports whether n is a primitive literal —
// a constant value the developer typed directly. Used to identify
// `const X = 1` (stable across renders) vs `const X = f()` (whose
// value can change).
func isLiteralExpression(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindRegularExpressionLiteral:
		return true
	}
	return false
}

// surroundingBindingName returns the identifier the callback's
// containing hook call (useCallback / useMemo) is being assigned to,
// if any. Walks: callback -> CallExpression -> VariableDeclaration
// or BinaryExpression(=).
func surroundingBindingName(callback *wrapperchecker.Node) string {
	p := callback.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindCallExpression {
		return ""
	}
	pp := p.Parent()
	if pp == nil {
		return ""
	}
	switch pp.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		var first *wrapperchecker.Node
		pp.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
				return true
			}
			return false
		})
		if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
			return first.LiteralText()
		}
	}
	return ""
}

func enclosingFunctionLike(n *wrapperchecker.Node) *wrapperchecker.Node {
	cur := n.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration:
			return cur
		}
		cur = cur.Parent()
	}
	return nil
}

func isReactiveHook(call *wrapperchecker.Node) bool {
	callee := call.CalleeExpression()
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return reactiveHooks[callee.LiteralText()]
	case wrapperchecker.KindPropertyAccessExpression:
		return reactiveHooks[callee.PropertyAccessName()]
	}
	return false
}

// hookCallbackAndDeps returns the (callback, depsArray, depsNode)
// triple from a reactive hook call:
//   - callback: the resolved arrow / function expression callback, or
//     nil when the first argument isn't function-shaped.
//   - depsArray: the literal array of dependencies, or nil when the
//     second argument is anything else (a variable, a function call,
//     a spread, etc.). depsNode is non-nil in that case so the rule
//     can anchor a diagnostic on the actual second argument.
//   - depsNode: the second argument as written.
func hookCallbackAndDeps(call *wrapperchecker.Node) (*wrapperchecker.Node, *wrapperchecker.Node, *wrapperchecker.Node) {
	args := callArguments(call)
	if len(args) < 2 {
		return nil, nil, nil
	}
	cb := unwrap(args[0])
	depsArg := args[1]
	deps := unwrap(depsArg)
	if cb == nil {
		return nil, nil, depsArg
	}
	switch cb.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		// ok
	default:
		return nil, nil, depsArg
	}
	if deps == nil || deps.Kind() != wrapperchecker.KindArrayLiteralExpression {
		return cb, nil, depsArg
	}
	return cb, deps, depsArg
}

func callArguments(call *wrapperchecker.Node) []*wrapperchecker.Node {
	seenCallee := false
	var out []*wrapperchecker.Node
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenCallee {
			seenCallee = true
			return false
		}
		// Skip type arguments — they appear between callee and value args.
		switch c.Kind() {
		case wrapperchecker.KindTypeReference, wrapperchecker.KindTypeLiteral:
			return false
		}
		out = append(out, c)
		return false
	})
	return out
}

// propertyAccessHead walks a chain of property / element accesses
// and TypeScript-only wrappers (non-null `!`, parens, `as T`, type
// assertions, `satisfies`) down to the leftmost identifier and
// returns its name. Returns "" if the head isn't an identifier (a
// call result, a `this`, etc.).
func propertyAccessHead(n *wrapperchecker.Node) string {
	cur := n
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindPropertyAccessExpression,
			wrapperchecker.KindElementAccessExpression:
			var inner *wrapperchecker.Node
			cur.ForEachChild(func(c *wrapperchecker.Node) bool {
				if inner == nil {
					inner = c
					return true
				}
				return false
			})
			if inner == nil {
				return ""
			}
			cur = inner
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindNonNullExpression,
			wrapperchecker.KindAsExpression,
			wrapperchecker.KindTypeAssertionExpression,
			wrapperchecker.KindSatisfiesExpression:
			var inner *wrapperchecker.Node
			cur.ForEachChild(func(c *wrapperchecker.Node) bool {
				if inner == nil && !isTypeNode(c) {
					inner = c
					return true
				}
				return false
			})
			if inner == nil {
				return ""
			}
			cur = inner
		default:
			if cur.Kind() == wrapperchecker.KindIdentifier {
				return cur.LiteralText()
			}
			return ""
		}
	}
	return ""
}

// freeIdentifiers walks the callback body collecting identifier
// references that are NOT declared inside the callback itself. The
// pass is intentionally syntactic: it treats every Identifier in
// expression position as a use and subtracts any name introduced by
// a local declaration (var/let/const, function parameters, function
// declarations) inside the callback's own scope tree.
func freeIdentifiers(callback *wrapperchecker.Node) []*wrapperchecker.Node {
	body := functionBody(callback)
	if body == nil {
		return nil
	}
	declared := map[string]bool{}
	collectLocalDeclarations(callback, declared)
	collectLocalDeclarations(body, declared)
	// A useCallback/useMemo result is typically assigned to a
	// variable (`const fib = useCallback(...)`). Inside the
	// callback the binding refers to itself — useful for recursion.
	// React guarantees the callback identity is stable across the
	// same render's lifetime, so a self-reference doesn't require
	// the binding to be in deps.
	if name := surroundingBindingName(callback); name != "" {
		declared[name] = true
	}
	var refs []*wrapperchecker.Node
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if isTypeNode(n) || isTypeQuery(n) {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindIdentifier:
			if isReferencingIdentifier(n) && !declared[n.LiteralText()] {
				refs = append(refs, n)
			}
			return
		case wrapperchecker.KindPropertyAccessExpression:
			var inner *wrapperchecker.Node
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if inner == nil {
					inner = c
					return true
				}
				return false
			})
			walk(inner)
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return dedupeByText(refs)
}

// dedupeByText keeps only the first occurrence of each identifier
// name. The rule reports at most one diagnostic per missing dep.
func dedupeByText(refs []*wrapperchecker.Node) []*wrapperchecker.Node {
	seen := map[string]bool{}
	var out []*wrapperchecker.Node
	for _, r := range refs {
		name := r.LiteralText()
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, r)
	}
	return out
}

// isReferencingIdentifier filters out identifiers that appear in
// non-reference positions (property names, parameter names, type
// references). A reference is an identifier that resolves to a
// value binding the surrounding expression reads from.
func isReferencingIdentifier(id *wrapperchecker.Node) bool {
	p := id.Parent()
	if p == nil {
		return true
	}
	switch p.Kind() {
	case wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment:
		// Property name slot: the first identifier child.
		var first *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
				return true
			}
			return false
		})
		if first != nil && first.Pos() == id.Pos() && first.End() == id.End() {
			// Shorthand `{ name }` IS a reference; PropertyAssignment
			// `{ name: value }` is not.
			return p.Kind() == wrapperchecker.KindShorthandPropertyAssignment
		}
	case wrapperchecker.KindPropertyAccessExpression:
		// Only the head identifier is a reference. The tail (property
		// name) is not.
		var first *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
				return true
			}
			return false
		})
		if first != nil && (first.Pos() != id.Pos() || first.End() != id.End()) {
			return false
		}
	case wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindEnumDeclaration:
		return false
	case wrapperchecker.KindTypeReference,
		wrapperchecker.KindQualifiedName:
		return false
	}
	return true
}

func functionBody(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindBlock:
			body = c
			return true
		}
		return false
	})
	if body != nil {
		return body
	}
	// Concise arrow body: take the last child.
	var last *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

// collectLocalDeclarations walks n looking for declarations that
// introduce names into the scope. Recursion into nested functions
// would discount their locals, but that's fine: any reference inside
// a nested function still has to come from somewhere, and `declared`
// only over-approximates locals (false-positives = miss a missing
// dep, never invent one).
func collectLocalDeclarations(n *wrapperchecker.Node, declared map[string]bool) {
	if n == nil {
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindParameter:
			for _, name := range bindingIdentifiers(c) {
				declared[name] = true
			}
		case wrapperchecker.KindVariableStatement:
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindVariableDeclarationList {
					g.ForEachChild(func(d *wrapperchecker.Node) bool {
						if d.Kind() == wrapperchecker.KindVariableDeclaration {
							for _, name := range bindingIdentifiers(d) {
								declared[name] = true
							}
						}
						return false
					})
				}
				return false
			})
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration:
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindIdentifier {
					declared[g.LiteralText()] = true
					return true
				}
				return false
			})
		case wrapperchecker.KindVariableDeclarationList:
			// Loop initializers (`for (let i = 0; ...)`,
			// `for (const x of arr)`) reach us as a bare list
			// without a VariableStatement wrapper.
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() == wrapperchecker.KindVariableDeclaration {
					for _, name := range bindingIdentifiers(d) {
						declared[name] = true
					}
				}
				return false
			})
		}
		collectLocalDeclarations(c, declared)
		return false
	})
}

func bindingIdentifiers(decl *wrapperchecker.Node) []string {
	var out []string
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindIdentifier:
			out = append(out, n.LiteralText())
		case wrapperchecker.KindObjectBindingPattern, wrapperchecker.KindArrayBindingPattern:
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c)
				return false
			})
		case wrapperchecker.KindBindingElement:
			walk(n.BindingElementName())
		}
	}
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if len(out) > 0 {
			return true
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			walk(c)
		}
		return false
	})
	return out
}

// unwrapInitializer peels TypeScript-only wrappers off the right-
// hand side of a const/let binding so a call to useRef wrapped in
// parens, non-null assertion, `as`, `satisfies`, or even a
// comma-expression (`(side, useRef())`) is still recognized as a
// stable-returning hook. For a comma expression the last operand
// is the produced value.
func unwrapInitializer(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil {
		switch n.Kind() {
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindNonNullExpression,
			wrapperchecker.KindAsExpression,
			wrapperchecker.KindTypeAssertionExpression,
			wrapperchecker.KindSatisfiesExpression:
			var inner *wrapperchecker.Node
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if inner == nil && !isTypeNode(c) {
					inner = c
					return true
				}
				return false
			})
			if inner == nil {
				return n
			}
			n = inner
			continue
		case wrapperchecker.KindBinaryExpression:
			if n.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
				// Comma expression: the value is the last operand.
				var last *wrapperchecker.Node
				n.ForEachChild(func(c *wrapperchecker.Node) bool {
					last = c
					return false
				})
				if last == nil {
					return n
				}
				n = last
				continue
			}
		}
		return n
	}
	return n
}

func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner == nil {
			break
		}
		n = inner
	}
	return n
}
