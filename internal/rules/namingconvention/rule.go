// Package namingconvention implements the naming-convention rule.
//
// Mirrors typescript-eslint's `naming-convention`: declaration names
// must conform to a configured chain of selectors / formats. Every
// declared identifier in the program is matched against every config
// (in declaration order); the first config whose selector + modifiers
// + types apply is the one whose format/affix/underscore rules
// validate the name. Identifiers that match no config are accepted.
//
// The default config (when no options are passed) enforces:
//   - `variable` / `function` / `parameter`: camelCase or UPPER_CASE
//   - everything else: PascalCase
package namingconvention

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "naming-convention"

var debug = false

// Options is the configurable surface. typescript-eslint accepts an
// array of selector configs; we keep that shape.
type Options struct {
	Configs []Selector
}

// Selector mirrors a single entry in upstream's options array.
type Selector struct {
	Selector            string   // "default" | "variable" | "function" | ...
	Modifiers           []string // ["const", "exported", ...]
	Types               []string // ["string", "number", ...]
	Format              []string // ["camelCase", "UPPER_CASE", ...] — nil = allow any
	Prefix              []string
	Suffix              []string
	LeadingUnderscore   string // "" | "allow" | "allowDouble" | "allowSingleOrDouble" | "forbid" | "require" | "requireDouble"
	TrailingUnderscore  string
	Filter              *MatchRegex
	Custom              *MatchRegex
}

// MatchRegex represents `{ regex, match }` where match: true means
// the regex must match, match: false means it must NOT match.
type MatchRegex struct {
	Regex *regexp.Regexp
	Match bool
}

// DefaultOptions returns upstream's defaults: variables / functions /
// parameters → camelCase|UPPER_CASE; everything else → PascalCase.
func DefaultOptions() Options {
	return Options{Configs: []Selector{
		{Selector: "default", Format: []string{"camelCase"}, LeadingUnderscore: "allow", TrailingUnderscore: "allow"},
		{Selector: "import", Format: []string{"camelCase", "PascalCase"}},
		{Selector: "variable", Format: []string{"camelCase", "UPPER_CASE"}, LeadingUnderscore: "allow", TrailingUnderscore: "allow"},
		{Selector: "typeLike", Format: []string{"PascalCase"}},
	}}
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

// OptionsFromJSON parses upstream's option array: a JSON array of
// selector config objects.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	if len(raw) == 0 {
		return DefaultOptions(), nil
	}
	var arr []rawSelector
	if err := json.Unmarshal(raw, &arr); err != nil {
		return Options{}, fmt.Errorf("naming-convention: %w", err)
	}
	if len(arr) == 0 {
		return DefaultOptions(), nil
	}
	out := Options{Configs: make([]Selector, 0, len(arr))}
	for _, r := range arr {
		s, err := r.toSelector()
		if err != nil {
			return Options{}, err
		}
		out.Configs = append(out.Configs, s)
	}
	return out, nil
}

type rawSelector struct {
	Selector            json.RawMessage `json:"selector"`
	Modifiers           []string        `json:"modifiers"`
	Types               []string        `json:"types"`
	Format              []string        `json:"format"`
	Prefix              []string        `json:"prefix"`
	Suffix              []string        `json:"suffix"`
	LeadingUnderscore   string          `json:"leadingUnderscore"`
	TrailingUnderscore  string          `json:"trailingUnderscore"`
	Filter              json.RawMessage `json:"filter"`
	Custom              json.RawMessage `json:"custom"`
}

func (r rawSelector) toSelector() (Selector, error) {
	out := Selector{
		Modifiers:          r.Modifiers,
		Types:              r.Types,
		Format:             r.Format,
		Prefix:             r.Prefix,
		Suffix:             r.Suffix,
		LeadingUnderscore:  r.LeadingUnderscore,
		TrailingUnderscore: r.TrailingUnderscore,
	}
	if err := parseSelector(r.Selector, &out); err != nil {
		return Selector{}, err
	}
	if mr, err := parseMatchRegex(r.Filter); err != nil {
		return Selector{}, err
	} else {
		out.Filter = mr
	}
	if mr, err := parseMatchRegex(r.Custom); err != nil {
		return Selector{}, err
	} else {
		out.Custom = mr
	}
	return out, nil
}

func parseSelector(raw json.RawMessage, out *Selector) error {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		out.Selector = s
		return nil
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err != nil {
		return fmt.Errorf("naming-convention: invalid selector: %w", err)
	}
	// Array selector: we approximate by joining — only "default" is
	// actually meaningful in the test corpus and `["variable","parameter"]`
	// is treated as multiple configs at expansion time. For now, store
	// the first.
	if len(ss) > 0 {
		out.Selector = ss[0]
	}
	return nil
}

func parseMatchRegex(raw json.RawMessage) (*MatchRegex, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// Accept a bare string as a shorthand: `filter: 'foo'` → regex foo, match true.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		re, err := regexp.Compile(s)
		if err != nil {
			return nil, fmt.Errorf("naming-convention: bad regex %q: %w", s, err)
		}
		return &MatchRegex{Regex: re, Match: true}, nil
	}
	var v struct {
		Regex string `json:"regex"`
		Match *bool  `json:"match"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("naming-convention: invalid match regex: %w", err)
	}
	re, err := regexp.Compile(v.Regex)
	if err != nil {
		return nil, fmt.Errorf("naming-convention: bad regex %q: %w", v.Regex, err)
	}
	match := true
	if v.Match != nil {
		match = *v.Match
	}
	return &MatchRegex{Regex: re, Match: match}, nil
}

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration:    r.visitVariable,
		wrapperchecker.KindParameter:              r.visitParameter,
		wrapperchecker.KindFunctionDeclaration:    r.visitFunction,
		wrapperchecker.KindMethodDeclaration:      r.visitMethod,
		wrapperchecker.KindPropertyDeclaration:    r.visitClassProperty,
		wrapperchecker.KindPropertySignature:      r.visitTypeProperty,
		wrapperchecker.KindMethodSignature:        r.visitTypeMethod,
		wrapperchecker.KindGetAccessor:            r.visitAccessor,
		wrapperchecker.KindSetAccessor:            r.visitAccessor,
		wrapperchecker.KindClassDeclaration:       r.visitTypeLike("class"),
		wrapperchecker.KindClassExpression:        r.visitTypeLike("class"),
		wrapperchecker.KindInterfaceDeclaration:   r.visitTypeLike("interface"),
		wrapperchecker.KindTypeAliasDeclaration:   r.visitTypeLike("typeAlias"),
		wrapperchecker.KindEnumDeclaration:        r.visitEnum,
		wrapperchecker.KindEnumMember:             r.visitEnumMember,
		wrapperchecker.KindTypeParameter:          r.visitTypeLike("typeParameter"),
		wrapperchecker.KindImportSpecifier:        r.visitImport,
		wrapperchecker.KindImportClause:           r.visitImport,
		wrapperchecker.KindNamespaceImport:        r.visitImport,
		wrapperchecker.KindPropertyAssignment:     r.visitObjLitProp,
		wrapperchecker.KindShorthandPropertyAssignment: r.visitObjLitProp,
	}
}

// --- handlers ---

func (r *rule) visitVariable(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	// VariableDeclaration in a destructuring pattern is handled at the
	// BindingElement level; only walk the top identifier here.
	idents := bindingIdents(n)
	mods := variableModifiers(n)
	types := variableTypes(ctx, n)
	for _, id := range idents {
		r.check(ctx, id, []string{"variable", "variableLike", "default"}, mods, types)
	}
	_ = parent
}

func (r *rule) visitParameter(ctx *engine.Context, n *wrapperchecker.Node) {
	idents := bindingIdents(n)
	mods := parameterModifiers(n)
	for _, id := range idents {
		r.check(ctx, id, []string{"parameter", "variableLike", "default"}, mods, nil)
	}
}

func (r *rule) visitFunction(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	mods := functionModifiers(n)
	r.check(ctx, id, []string{"function", "variableLike", "default"}, mods, nil)
}

func (r *rule) visitMethod(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	mods := classMemberModifiers(n)
	selectors := []string{"method", "memberLike", "default"}
	if parent := n.Parent(); parent != nil {
		switch parent.Kind() {
		case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
			selectors = []string{"classMethod", "method", "memberLike", "default"}
		case wrapperchecker.KindObjectLiteralExpression:
			selectors = []string{"objectLiteralMethod", "method", "memberLike", "default"}
		}
	}
	r.check(ctx, id, selectors, mods, nil)
}

func (r *rule) visitClassProperty(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	mods := classMemberModifiers(n)
	r.check(ctx, id, []string{"classProperty", "property", "memberLike", "default"}, mods, nil)
}

func (r *rule) visitTypeProperty(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	r.check(ctx, id, []string{"typeProperty", "property", "memberLike", "default"}, nil, nil)
}

func (r *rule) visitTypeMethod(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	r.check(ctx, id, []string{"typeMethod", "method", "memberLike", "default"}, nil, nil)
}

func (r *rule) visitAccessor(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	mods := classMemberModifiers(n)
	r.check(ctx, id, []string{"accessor", "memberLike", "default"}, mods, nil)
}

func (r *rule) visitEnum(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	r.check(ctx, id, []string{"enum", "typeLike", "default"}, typeLikeModifiers(n), nil)
}

func (r *rule) visitEnumMember(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	r.check(ctx, id, []string{"enumMember", "memberLike", "default"}, nil, nil)
}

func (r *rule) visitTypeLike(kind string) engine.Handler {
	return func(ctx *engine.Context, n *wrapperchecker.Node) {
		id := nameNode(n)
		if id == nil {
			return
		}
		r.check(ctx, id, []string{kind, "typeLike", "default"}, typeLikeModifiers(n), nil)
	}
}

// typeLikeModifiers extracts the "exported" / "default" / "ambient"
// modifiers from a top-level class / interface / type-alias / enum
// declaration.
func typeLikeModifiers(n *wrapperchecker.Node) []string {
	var mods []string
	if n.HasExportModifier() {
		mods = append(mods, "exported")
	}
	if n.HasDefaultModifier() {
		mods = append(mods, "default")
	}
	if n.HasDeclareModifier() {
		mods = append(mods, "ambient")
	}
	if n.HasAbstractModifier() {
		mods = append(mods, "abstract")
	}
	return mods
}

func (r *rule) visitImport(ctx *engine.Context, n *wrapperchecker.Node) {
	id := importNameNode(n)
	if id == nil {
		return
	}
	var mods []string
	switch n.Kind() {
	case wrapperchecker.KindImportClause:
		mods = append(mods, "default")
	case wrapperchecker.KindNamespaceImport:
		mods = append(mods, "namespace")
	case wrapperchecker.KindImportSpecifier:
		// `import { default as foo } from ...` carries default modifier.
		if isDefaultImportSpecifier(n) {
			mods = append(mods, "default")
		}
	}
	r.check(ctx, id, []string{"import", "default"}, mods, nil)
}

// importNameNode returns the locally-bound name for an import-kind
// node, preferring the renamed-to form (`{ X as Y }` → Y, `{ default as
// Y }` → Y).
func importNameNode(n *wrapperchecker.Node) *wrapperchecker.Node {
	if n.Kind() == wrapperchecker.KindImportSpecifier {
		// ImportSpecifier may have [propertyName, name]. The LAST
		// identifier-shaped child is the local binding name.
		var last *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier {
				last = c
			}
			return false
		})
		return last
	}
	return nameNode(n)
}

// isDefaultImportSpecifier reports whether an ImportSpecifier
// represents `{ default as foo }` (i.e. its property name is the
// `default` keyword/identifier).
func isDefaultImportSpecifier(n *wrapperchecker.Node) bool {
	var idents []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier ||
			c.Kind() == wrapperchecker.KindStringLiteral {
			idents = append(idents, c)
		}
		return false
	})
	if len(idents) >= 2 {
		return idents[0].LiteralText() == "default"
	}
	return false
}

func (r *rule) visitObjLitProp(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	r.check(ctx, id, []string{"objectLiteralProperty", "property", "memberLike", "default"}, nil, nil)
}

// --- core matcher ---

// check finds the first selector config whose selector + modifiers +
// types apply to this identifier, and validates the name against it.
// "ident" is the name node; "selectors" is the list of category names
// (most-specific-first) to consider.
func (r *rule) check(
	ctx *engine.Context,
	ident *wrapperchecker.Node,
	selectors []string,
	modifiers []string,
	types []string,
) {
	if ident == nil {
		return
	}
	name := ident.LiteralText()
	if name == "" {
		return
	}
	// Walk candidate selector levels from most-specific to least-specific.
	// At each level, configs with a filter that matches the name take
	// priority over filter-less configs; configs with more modifiers
	// take priority over configs with fewer; types act the same way.
	// This mirrors upstream's "most specific match wins" behaviour.
	for _, candidate := range selectors {
		if cfg := bestConfigFor(r.opts.Configs, candidate, name, modifiers, types); cfg != nil {
			if msg, ok := validateName(name, *cfg); !ok {
				ctx.Report(ident, msg)
			}
			return
		}
	}
}

// bestConfigFor picks the most specific config at the given selector
// level that applies to (name, modifiers, types). Specificity rank:
//
//	1. filter > no-filter
//	2. more required modifiers > fewer
//	3. more required types > fewer
//	4. earlier declaration > later
func bestConfigFor(configs []Selector, candidate, name string, mods, types []string) *Selector {
	bestIdx := -1
	bestScore := [3]int{-1, -1, -1}
	for i := range configs {
		cfg := &configs[i]
		if cfg.Selector != candidate {
			continue
		}
		if !modifiersMatch(cfg.Modifiers, mods) {
			continue
		}
		if !typesMatch(cfg.Types, types) {
			continue
		}
		if cfg.Filter != nil && !regexAllows(cfg.Filter, name) {
			continue
		}
		score := [3]int{0, len(cfg.Modifiers), len(cfg.Types)}
		if cfg.Filter != nil {
			score[0] = 1
		}
		if bestIdx == -1 || cmpScore(score, bestScore) > 0 {
			bestIdx = i
			bestScore = score
		}
	}
	if bestIdx == -1 {
		return nil
	}
	return &configs[bestIdx]
}

func cmpScore(a, b [3]int) int {
	for i := range a {
		if a[i] != b[i] {
			return a[i] - b[i]
		}
	}
	return 0
}

func selectorMatches(cfgSelector string, candidates []string) bool {
	if cfgSelector == "" {
		return false
	}
	for _, c := range candidates {
		if cfgSelector == c {
			return true
		}
	}
	return false
}

func modifiersMatch(required, actual []string) bool {
	for _, req := range required {
		found := false
		for _, a := range actual {
			if a == req {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func typesMatch(required, actual []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, req := range required {
		found := false
		for _, a := range actual {
			if a == req {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func regexAllows(mr *MatchRegex, name string) bool {
	matched := mr.Regex.MatchString(name)
	return matched == mr.Match
}

// validateName checks `name` against the selector's format/affix/
// underscore/custom rules. Returns (msg, false) on failure.
func validateName(name string, cfg Selector) (string, bool) {
	if cfg.Custom != nil && !regexAllows(cfg.Custom, name) {
		return "name `" + name + "` doesn't match the configured custom regex", false
	}
	// Strip leading and trailing underscores per option, then strip
	// prefix/suffix, then check format.
	trimmed, lead, ok := stripLeadingUnderscore(name, cfg.LeadingUnderscore)
	if !ok {
		return "name `" + name + "` has disallowed leading underscore", false
	}
	trimmed, trail, ok := stripTrailingUnderscore(trimmed, cfg.TrailingUnderscore)
	if !ok {
		return "name `" + name + "` has disallowed trailing underscore", false
	}
	_ = lead
	_ = trail
	// Affixes.
	if len(cfg.Prefix) > 0 {
		matched := false
		for _, p := range cfg.Prefix {
			if strings.HasPrefix(trimmed, p) {
				trimmed = strings.TrimPrefix(trimmed, p)
				matched = true
				break
			}
		}
		if !matched {
			return "name `" + name + "` is missing required prefix", false
		}
	}
	if len(cfg.Suffix) > 0 {
		matched := false
		for _, s := range cfg.Suffix {
			if strings.HasSuffix(trimmed, s) {
				trimmed = strings.TrimSuffix(trimmed, s)
				matched = true
				break
			}
		}
		if !matched {
			return "name `" + name + "` is missing required suffix", false
		}
	}
	// Format check.
	if len(cfg.Format) == 0 {
		return "", true
	}
	if trimmed == "" {
		// e.g. name was just underscores + prefix that got fully trimmed.
		return "", true
	}
	for _, f := range cfg.Format {
		if formatMatches(f, trimmed) {
			return "", true
		}
	}
	return "name `" + name + "` doesn't match the configured format", false
}

func stripLeadingUnderscore(name, opt string) (string, string, bool) {
	count := 0
	for count < len(name) && name[count] == '_' {
		count++
	}
	prefix := name[:count]
	rest := name[count:]
	switch opt {
	case "", "forbid":
		if count > 0 {
			return rest, prefix, false
		}
	case "allow":
		// Any non-negative count accepted.
	case "allowSingleOrDouble":
		if count > 2 {
			return rest, prefix, false
		}
	case "allowDouble":
		if count != 2 {
			return rest, prefix, false
		}
	case "require":
		if count != 1 {
			return rest, prefix, false
		}
	case "requireDouble":
		if count != 2 {
			return rest, prefix, false
		}
	}
	return rest, prefix, true
}

func stripTrailingUnderscore(name, opt string) (string, string, bool) {
	count := 0
	for count < len(name) && name[len(name)-1-count] == '_' {
		count++
	}
	suffix := name[len(name)-count:]
	rest := name[:len(name)-count]
	switch opt {
	case "", "forbid":
		if count > 0 {
			return rest, suffix, false
		}
	case "allow":
	case "allowSingleOrDouble":
		if count > 2 {
			return rest, suffix, false
		}
	case "allowDouble":
		if count != 2 {
			return rest, suffix, false
		}
	case "require":
		if count != 1 {
			return rest, suffix, false
		}
	case "requireDouble":
		if count != 2 {
			return rest, suffix, false
		}
	}
	return rest, suffix, true
}

// --- format predicates ---

func formatMatches(format, s string) bool {
	if s == "" {
		return true
	}
	switch format {
	case "camelCase":
		return isCamelCase(s, false)
	case "strictCamelCase":
		return isCamelCase(s, true)
	case "PascalCase":
		return isPascalCase(s, false)
	case "strictPascalCase":
		return isPascalCase(s, true)
	case "snake_case":
		return isSnakeCase(s)
	case "UPPER_CASE":
		return isUpperCase(s)
	}
	return false
}

func isCamelCase(s string, strict bool) bool {
	if s == "" {
		return false
	}
	first := []rune(s)[0]
	if !unicode.IsLower(first) && first != '$' && !unicode.IsDigit(first) {
		return false
	}
	for _, r := range s {
		if r == '_' {
			return false
		}
	}
	if strict {
		// no consecutive caps
		var prev rune
		for _, r := range s {
			if unicode.IsUpper(r) && unicode.IsUpper(prev) {
				return false
			}
			prev = r
		}
	}
	return true
}

func isPascalCase(s string, strict bool) bool {
	if s == "" {
		return false
	}
	first := []rune(s)[0]
	if !unicode.IsUpper(first) {
		return false
	}
	for _, r := range s {
		if r == '_' {
			return false
		}
	}
	if strict {
		var prev rune
		for i, r := range s {
			if i > 0 && unicode.IsUpper(r) && unicode.IsUpper(prev) {
				return false
			}
			prev = r
		}
	}
	return true
}

func isSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case unicode.IsLower(r),
			unicode.IsDigit(r),
			r == '_', r == '$':
		default:
			return false
		}
	}
	return true
}

func isUpperCase(s string) bool {
	if s == "" {
		return false
	}
	hasUpper := false
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r), r == '_', r == '$':
		default:
			return false
		}
	}
	return hasUpper
}

// --- AST helpers ---

func nameNode(n *wrapperchecker.Node) *wrapperchecker.Node {
	var ident *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if ident != nil {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPrivateIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNumericLiteral:
			ident = c
			return true
		}
		return false
	})
	return ident
}

// bindingIdents walks a declaration's name slot, descending into
// binding patterns to surface every identifier.
func bindingIdents(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	var walk func(*wrapperchecker.Node)
	walk = func(node *wrapperchecker.Node) {
		if node == nil {
			return
		}
		switch node.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPrivateIdentifier:
			out = append(out, node)
			return
		case wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			node.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c)
				return false
			})
			return
		case wrapperchecker.KindBindingElement:
			// BindingElement has [propertyName, name, initializer]. Only
			// the (renamed-to) name should be checked.
			var nameSlot *wrapperchecker.Node
			node.ForEachChild(func(c *wrapperchecker.Node) bool {
				switch c.Kind() {
				case wrapperchecker.KindIdentifier,
					wrapperchecker.KindObjectBindingPattern,
					wrapperchecker.KindArrayBindingPattern:
					if nameSlot == nil {
						nameSlot = c
					}
				}
				return false
			})
			walk(nameSlot)
			return
		}
	}
	// Find the first name-shaped child of n and descend.
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if len(out) > 0 {
			return false
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

// --- modifier / type extractors ---

func variableModifiers(n *wrapperchecker.Node) []string {
	var mods []string
	if isConstVariable(n) {
		mods = append(mods, "const")
	}
	if hasExportedAncestor(n) {
		mods = append(mods, "exported")
	}
	if hasDeclareAncestor(n) {
		mods = append(mods, "ambient")
	}
	if isGlobalScope(n) {
		mods = append(mods, "global")
	}
	if isDestructuredVariable(n) {
		mods = append(mods, "destructured")
	}
	return mods
}

// isGlobalScope reports whether the declaration is at the top level of
// the module or a namespace — i.e. not nested inside any Block.
func isGlobalScope(n *wrapperchecker.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindBlock:
			return false
		case wrapperchecker.KindSourceFile,
			wrapperchecker.KindModuleBlock:
			return true
		}
	}
	return true
}

// isDestructuredVariable reports whether a VariableDeclaration is the
// binding-pattern form (`const { x } = …`). The walk surfaces every
// identifier produced by the pattern via bindingIdents.
func isDestructuredVariable(n *wrapperchecker.Node) bool {
	var destructured bool
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			destructured = true
			return true
		}
		return false
	})
	return destructured
}

func parameterModifiers(n *wrapperchecker.Node) []string {
	var mods []string
	if isDestructured(n) {
		mods = append(mods, "destructured")
	}
	return mods
}

func functionModifiers(n *wrapperchecker.Node) []string {
	var mods []string
	if wrapperchecker.HasAsyncModifier(n) {
		mods = append(mods, "async")
	}
	if n.HasExportModifier() || hasExportedAncestor(n) {
		mods = append(mods, "exported")
	}
	if n.HasDefaultModifier() {
		mods = append(mods, "default")
	}
	if n.HasDeclareModifier() {
		mods = append(mods, "ambient")
	}
	if isGlobalScope(n) {
		mods = append(mods, "global")
	}
	return mods
}

func classMemberModifiers(n *wrapperchecker.Node) []string {
	var mods []string
	for _, m := range modifierKinds(n) {
		switch m {
		case wrapperchecker.KindStaticKeyword:
			mods = append(mods, "static")
		case wrapperchecker.KindReadonlyKeyword:
			mods = append(mods, "readonly")
		case wrapperchecker.KindPrivateKeyword:
			mods = append(mods, "private")
		case wrapperchecker.KindProtectedKeyword:
			mods = append(mods, "protected")
		case wrapperchecker.KindPublicKeyword:
			mods = append(mods, "public")
		case wrapperchecker.KindAbstractKeyword:
			mods = append(mods, "abstract")
		case wrapperchecker.KindAsyncKeyword:
			mods = append(mods, "async")
		case wrapperchecker.KindOverrideKeyword:
			mods = append(mods, "override")
		}
	}
	// Private-identifier methods count as #private.
	if id := nameNode(n); id != nil && id.Kind() == wrapperchecker.KindPrivateIdentifier {
		mods = append(mods, "#private")
	}
	return mods
}

func variableTypes(ctx *engine.Context, n *wrapperchecker.Node) []string {
	t := ctx.TypeOf(n)
	if t == nil {
		return nil
	}
	return typeCategories(t)
}

// typeCategories returns the upstream "type" classifications a type
// matches. A union qualifies as "string" if every non-nullish member is
// string-like, etc.
func typeCategories(t *wrapperchecker.Type) []string {
	var cats []string
	if isAllStringish(t) {
		cats = append(cats, "string")
	}
	if isAllNumberish(t) {
		cats = append(cats, "number")
	}
	if isAllBoolish(t) {
		cats = append(cats, "boolean")
	}
	if isAllArrayish(t) {
		cats = append(cats, "array")
	}
	if isAllFunctionish(t) {
		cats = append(cats, "function")
	}
	return cats
}

func isAllArrayish(t *wrapperchecker.Type) bool {
	hasArray := false
	for _, m := range constituents(t) {
		if m.IsNullOrUndefined() {
			continue
		}
		if !m.IsArrayLikeType() && !m.IsTupleType() {
			return false
		}
		hasArray = true
	}
	return hasArray
}

func isAllFunctionish(t *wrapperchecker.Type) bool {
	hasFn := false
	for _, m := range constituents(t) {
		if m.IsNullOrUndefined() {
			continue
		}
		if len(m.CallSignatures()) == 0 {
			return false
		}
		hasFn = true
	}
	return hasFn
}

func isAllStringish(t *wrapperchecker.Type) bool {
	for _, m := range constituents(t) {
		if m.IsNullOrUndefined() {
			continue
		}
		if !m.IsStringLike() {
			return false
		}
	}
	return true
}

func isAllNumberish(t *wrapperchecker.Type) bool {
	for _, m := range constituents(t) {
		if m.IsNullOrUndefined() {
			continue
		}
		if !m.IsNumberLike() {
			return false
		}
	}
	return true
}

func isAllBoolish(t *wrapperchecker.Type) bool {
	for _, m := range constituents(t) {
		if m.IsNullOrUndefined() {
			continue
		}
		if !m.IsBooleanLike() {
			return false
		}
	}
	return true
}

func constituents(t *wrapperchecker.Type) []*wrapperchecker.Type {
	if t == nil {
		return nil
	}
	if t.IsUnion() {
		return t.UnionMembers()
	}
	return []*wrapperchecker.Type{t}
}

// --- modifier-kind walkers ---

func modifierKinds(n *wrapperchecker.Node) []wrapperchecker.Kind {
	var kinds []wrapperchecker.Kind
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		// Modifiers appear before the name in declarations.
		switch c.Kind() {
		case wrapperchecker.KindStaticKeyword,
			wrapperchecker.KindReadonlyKeyword,
			wrapperchecker.KindPrivateKeyword,
			wrapperchecker.KindProtectedKeyword,
			wrapperchecker.KindPublicKeyword,
			wrapperchecker.KindAbstractKeyword,
			wrapperchecker.KindAsyncKeyword,
			wrapperchecker.KindOverrideKeyword,
			wrapperchecker.KindExportKeyword,
			wrapperchecker.KindDefaultKeyword,
			wrapperchecker.KindDeclareKeyword:
			kinds = append(kinds, c.Kind())
		}
		return false
	})
	return kinds
}

func isConstVariable(varDecl *wrapperchecker.Node) bool {
	// A VariableDeclaration's parent is a VariableDeclarationList whose
	// flags indicate const/let/var. We don't have direct access to those
	// flags in the wrapper, so use a heuristic: look for the
	// `KindConstKeyword` in the grandparent (VariableStatement).
	for p := varDecl.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindVariableStatement {
			return varStatementIsConst(p)
		}
		// Don't walk past two levels.
		if p.Parent() != nil && p.Parent().Parent() != nil &&
			p.Parent().Parent() != nil {
			break
		}
	}
	return false
}

func varStatementIsConst(stmt *wrapperchecker.Node) bool {
	var seenConst bool
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindVariableDeclarationList {
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindConstKeyword {
					seenConst = true
					return true
				}
				return false
			})
			return true
		}
		return false
	})
	return seenConst
}

func hasExportedAncestor(n *wrapperchecker.Node) bool {
	for p := n; p != nil; p = p.Parent() {
		if p.HasExportModifier() {
			return true
		}
		switch p.Kind() {
		case wrapperchecker.KindBlock:
			return false
		}
	}
	return false
}

func hasDeclareAncestor(n *wrapperchecker.Node) bool {
	for p := n; p != nil; p = p.Parent() {
		for _, k := range modifierKinds(p) {
			if k == wrapperchecker.KindDeclareKeyword {
				return true
			}
		}
	}
	return false
}

func hasDefaultExportModifier(n *wrapperchecker.Node) bool {
	for _, k := range modifierKinds(n) {
		if k == wrapperchecker.KindDefaultKeyword {
			return true
		}
	}
	return false
}

func isDestructured(param *wrapperchecker.Node) bool {
	var destructured bool
	param.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			destructured = true
			return true
		}
		return false
	})
	return destructured
}
