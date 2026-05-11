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
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "naming-convention"

// Options is the configurable surface. typescript-eslint accepts an
// array of selector configs; we keep that shape.
type Options struct {
	Configs []Selector
}

// Selector mirrors a single entry in upstream's options array.
type Selector struct {
	Selectors          []string // one or more of "default" | "variable" | ...
	Modifiers          []string // ["const", "exported", ...]
	Types              []string // ["string", "number", ...]
	Format             []string // ["camelCase", "UPPER_CASE", ...] — nil = allow any
	FormatNull         bool     // true when `format: null` (accept any)
	Prefix             []string
	Suffix             []string
	LeadingUnderscore  string // "" | "allow" | "allowDouble" | "allowSingleOrDouble" | "forbid" | "require" | "requireDouble"
	TrailingUnderscore string
	Filter             *MatchRegex
	Custom             *MatchRegex
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
		{Selectors: []string{"default"}, Format: []string{"camelCase"}, LeadingUnderscore: "allow", TrailingUnderscore: "allow"},
		{Selectors: []string{"import"}, Format: []string{"camelCase", "PascalCase"}},
		{Selectors: []string{"variable"}, Format: []string{"camelCase", "UPPER_CASE"}, LeadingUnderscore: "allow", TrailingUnderscore: "allow"},
		{Selectors: []string{"typeLike"}, Format: []string{"PascalCase"}},
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
	Selector           json.RawMessage `json:"selector"`
	Modifiers          []string        `json:"modifiers"`
	Types              []string        `json:"types"`
	Format             json.RawMessage `json:"format"`
	Prefix             []string        `json:"prefix"`
	Suffix             []string        `json:"suffix"`
	LeadingUnderscore  string          `json:"leadingUnderscore"`
	TrailingUnderscore string          `json:"trailingUnderscore"`
	Filter             json.RawMessage `json:"filter"`
	Custom             json.RawMessage `json:"custom"`
}

func (r rawSelector) toSelector() (Selector, error) {
	out := Selector{
		Modifiers:          r.Modifiers,
		Types:              r.Types,
		Prefix:             r.Prefix,
		Suffix:             r.Suffix,
		LeadingUnderscore:  r.LeadingUnderscore,
		TrailingUnderscore: r.TrailingUnderscore,
	}
	if err := parseSelector(r.Selector, &out); err != nil {
		return Selector{}, err
	}
	if err := parseFormat(r.Format, &out); err != nil {
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
		out.Selectors = []string{s}
		return nil
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err != nil {
		return fmt.Errorf("naming-convention: invalid selector: %w", err)
	}
	out.Selectors = ss
	return nil
}

// parseFormat handles `format: null` (no format constraint) and the
// usual `format: ["camelCase", ...]` array form.
func parseFormat(raw json.RawMessage, out *Selector) error {
	if len(raw) == 0 || string(raw) == "null" {
		out.FormatNull = true
		return nil
	}
	var fs []string
	if err := json.Unmarshal(raw, &fs); err != nil {
		return fmt.Errorf("naming-convention: invalid format: %w", err)
	}
	out.Format = fs
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

type rule struct {
	opts            Options
	usageCache      map[string]map[uintptr]int
	exportNameCache map[string]map[string]struct{}
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration:    r.visitVariable,
		wrapperchecker.KindParameter:              r.visitParameter,
		wrapperchecker.KindFunctionDeclaration:    r.visitFunction,
		wrapperchecker.KindFunctionExpression:     r.visitFunction,
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
	types := variableTypes(ctx, n)
	for _, id := range bindingIdents(n) {
		mods := r.variableModifiers(ctx, n, id)
		// `destructured` only applies to non-renamed bindings — when the
		// binding element specifies a different property name, the local
		// identifier was chosen by the developer and is treated as a
		// "regular" binding.
		if isRenamedBinding(id) {
			mods = removeMod(mods, "destructured")
		}
		r.check(ctx, id, []string{"variable", "variableLike", "default"}, mods, types)
	}
}

// isRenamedBinding reports whether idNode is the local-name slot of a
// BindingElement whose propertyName is explicitly set
// (`{ a: b } = …` → b is renamed).
func isRenamedBinding(idNode *wrapperchecker.Node) bool {
	if idNode == nil {
		return false
	}
	p := idNode.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindBindingElement {
		return false
	}
	return p.BindingElementPropertyName() != nil
}

// removeMod returns a new slice with all occurrences of want removed.
func removeMod(mods []string, want string) []string {
	out := mods[:0:0]
	for _, m := range mods {
		if m != want {
			out = append(out, m)
		}
	}
	return out
}

func (r *rule) visitParameter(ctx *engine.Context, n *wrapperchecker.Node) {
	mods := parameterModifiers(n)
	types := variableTypes(ctx, n)
	// A constructor parameter that carries an access/readonly modifier
	// (TypeScript "parameter property") is reported under the
	// parameterProperty selector and inherits those modifiers.
	paramPropMods := parameterPropertyModifiers(n)
	isParamProp := len(paramPropMods) > 0 && isConstructorParameter(n)
	for _, id := range bindingIdents(n) {
		thisMods := append([]string{}, mods...)
		thisMods = append(thisMods, paramPropMods...)
		if r.isUnused(ctx, id) {
			thisMods = append(thisMods, "unused")
		}
		if isRenamedBinding(id) {
			thisMods = removeMod(thisMods, "destructured")
		}
		selectors := []string{"parameter", "variableLike", "default"}
		if isParamProp {
			selectors = []string{"parameterProperty", "property", "memberLike", "default"}
		}
		r.check(ctx, id, selectors, thisMods, types)
	}
}

// parameterPropertyModifiers returns the access/readonly modifiers
// present on a constructor parameter that promote it to a class
// parameter property. Returns nil if no such modifiers are present.
func parameterPropertyModifiers(n *wrapperchecker.Node) []string {
	var mods []string
	for _, m := range modifierKinds(n) {
		switch m {
		case wrapperchecker.KindReadonlyKeyword:
			mods = append(mods, "readonly")
		case wrapperchecker.KindPrivateKeyword:
			mods = append(mods, "private")
		case wrapperchecker.KindProtectedKeyword:
			mods = append(mods, "protected")
		case wrapperchecker.KindPublicKeyword:
			mods = append(mods, "public")
		case wrapperchecker.KindOverrideKeyword:
			mods = append(mods, "override")
		}
	}
	return mods
}

func isConstructorParameter(n *wrapperchecker.Node) bool {
	p := n.Parent()
	return p != nil && p.Kind() == wrapperchecker.KindConstructor
}

func (r *rule) visitFunction(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	mods := functionModifiers(n)
	if r.isExportedViaBlock(ctx, id) && !containsMod(mods, "exported") {
		mods = append(mods, "exported")
	}
	if r.isUnused(ctx, id) {
		mods = append(mods, "unused")
	}
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
	types := variableTypes(ctx, n)
	r.check(ctx, id, []string{"classProperty", "property", "memberLike", "default"}, mods, types)
}

func (r *rule) visitTypeProperty(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	var mods []string
	if nameRequiresQuotes(id) {
		mods = append(mods, "requiresQuotes")
	}
	// A property signature whose annotation is a function type is
	// classified as typeMethod, matching upstream.
	if propertyTypeIsFunction(n) {
		r.check(ctx, id, []string{"typeMethod", "method", "memberLike", "default"}, mods, nil)
		return
	}
	r.check(ctx, id, []string{"typeProperty", "property", "memberLike", "default"}, mods, nil)
}

// propertyTypeIsFunction reports whether a PropertySignature has a
// function-type annotation (`name: () => T`).
func propertyTypeIsFunction(n *wrapperchecker.Node) bool {
	var hasFn bool
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindFunctionType:
			hasFn = true
			return true
		}
		return false
	})
	return hasFn
}

func (r *rule) visitTypeMethod(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	var mods []string
	if nameRequiresQuotes(id) {
		mods = append(mods, "requiresQuotes")
	}
	r.check(ctx, id, []string{"typeMethod", "method", "memberLike", "default"}, mods, nil)
}

func (r *rule) visitAccessor(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	// classMemberModifiers already records requiresQuotes for class
	// bodies; for object-literal accessors, fall back to the
	// node-by-node lookup.
	var mods []string
	parent := n.Parent()
	if parent != nil && parent.Kind() == wrapperchecker.KindObjectLiteralExpression {
		if nameRequiresQuotes(id) {
			mods = append(mods, "requiresQuotes")
		}
	} else {
		mods = classMemberModifiers(n)
	}
	r.check(ctx, id, []string{"accessor", "memberLike", "default"}, mods, nil)
}

func (r *rule) visitEnum(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	mods := typeLikeModifiers(n)
	if r.isExportedViaBlock(ctx, id) && !containsMod(mods, "exported") {
		mods = append(mods, "exported")
	}
	if r.isUnused(ctx, id) {
		mods = append(mods, "unused")
	}
	r.check(ctx, id, []string{"enum", "typeLike", "default"}, mods, nil)
}

func (r *rule) visitEnumMember(ctx *engine.Context, n *wrapperchecker.Node) {
	id := nameNode(n)
	if id == nil {
		return
	}
	var mods []string
	if nameRequiresQuotes(id) {
		mods = append(mods, "requiresQuotes")
	}
	r.check(ctx, id, []string{"enumMember", "memberLike", "default"}, mods, nil)
}

func (r *rule) visitTypeLike(kind string) engine.Handler {
	return func(ctx *engine.Context, n *wrapperchecker.Node) {
		id := nameNode(n)
		if id == nil {
			return
		}
		mods := typeLikeModifiers(n)
		if r.isExportedViaBlock(ctx, id) && !containsMod(mods, "exported") {
			mods = append(mods, "exported")
		}
		if r.isUnused(ctx, id) {
			mods = append(mods, "unused")
		}
		r.check(ctx, id, []string{kind, "typeLike", "default"}, mods, nil)
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
		} else if !isAliasedImportSpecifier(n) {
			// `import { foo_bar } from ...` (no alias) — the local
			// binding name is dictated by the imported module and is
			// not under the consumer's control. typescript-eslint
			// skips these.
			return
		}
	}
	r.check(ctx, id, []string{"import", "default"}, mods, nil)
}

// isAliasedImportSpecifier reports whether an ImportSpecifier has the
// form `{ name as alias }` (two distinct identifier children).
func isAliasedImportSpecifier(n *wrapperchecker.Node) bool {
	count := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			count++
		}
		return false
	})
	return count >= 2
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
	var mods []string
	if nameRequiresQuotes(id) {
		mods = append(mods, "requiresQuotes")
	}
	r.check(ctx, id, []string{"objectLiteralProperty", "property", "memberLike", "default"}, mods, nil)
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
		var best *Selector
		bestScore := -1
		for i := range r.opts.Configs {
			cfg := &r.opts.Configs[i]
			if !cfgSelectsCandidate(cfg, candidate) {
				continue
			}
			if !modifiersMatch(cfg.Modifiers, modifiers) {
				continue
			}
			if !typesMatch(cfg.Types, types) {
				continue
			}
			if cfg.Filter != nil && !regexAllows(cfg.Filter, name) {
				continue
			}
			if !affixesApply(name, cfg) {
				continue
			}
			s := specificity(cfg)
			if best == nil || s > bestScore {
				best = cfg
				bestScore = s
			}
		}
		if best == nil {
			continue
		}
		if msg, ok := validateName(name, *best); !ok {
			ctx.Report(ident, msg)
		}
		return
	}
}

// specificity scores a config by its number of independent constraints
// (modifiers + types + filter + custom + affixes). Used to break ties
// between multiple applicable configs at a single selector level.
func specificity(cfg *Selector) int {
	score := len(cfg.Modifiers) + len(cfg.Types) + len(cfg.Prefix) + len(cfg.Suffix)
	if cfg.Filter != nil {
		score++
	}
	if cfg.Custom != nil {
		score++
	}
	return score
}

// affixesApply reports whether the prefix / suffix / leadingUnderscore /
// trailingUnderscore constraints of cfg admit the given name. When any
// of them is required and absent, the config falls through to the next
// candidate rather than producing an error.
func affixesApply(name string, cfg *Selector) bool {
	trimmed, _, ok := stripLeadingUnderscore(name, cfg.LeadingUnderscore)
	if !ok {
		return false
	}
	trimmed, _, ok = stripTrailingUnderscore(trimmed, cfg.TrailingUnderscore)
	if !ok {
		return false
	}
	if len(cfg.Prefix) > 0 {
		matched := false
		for _, p := range cfg.Prefix {
			if strings.HasPrefix(trimmed, p) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(cfg.Suffix) > 0 {
		matched := false
		for _, s := range cfg.Suffix {
			if strings.HasSuffix(trimmed, s) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
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
		if !cfgSelectsCandidate(cfg, candidate) {
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

// cfgSelectsCandidate reports whether the config applies to the given
// selector category (one of "default", "variable", ...). A config's
// Selectors list is matched against the candidate, which may be a
// concrete category like "variable" or a group like "variableLike".
func cfgSelectsCandidate(cfg *Selector, candidate string) bool {
	for _, s := range cfg.Selectors {
		if s == candidate {
			return true
		}
	}
	return false
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
	// For declarations whose type wasn't categorised (functions, class
	// declarations, imports, ...), the `types` constraint is treated as
	// "ignored" — upstream's `types` option is only meaningful for
	// typed-value selectors (variable/parameter/property/etc.).
	if len(actual) == 0 {
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
// underscore/custom rules. Returns (msg, false) on failure. The check
// peels off leading/trailing underscores and prefix/suffix first; the
// resulting "processed" name is what the custom regex and format
// validators see.
func validateName(name string, cfg Selector) (string, bool) {
	// Strip the leading `#` from private-identifier names; only the
	// bare identifier participates in format validation.
	if strings.HasPrefix(name, "#") {
		name = name[1:]
	}
	trimmed, _, ok := stripLeadingUnderscore(name, cfg.LeadingUnderscore)
	if !ok {
		return "name `" + name + "` has disallowed leading underscore", false
	}
	trimmed, _, ok = stripTrailingUnderscore(trimmed, cfg.TrailingUnderscore)
	if !ok {
		return "name `" + name + "` has disallowed trailing underscore", false
	}
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
	if cfg.Custom != nil && !regexAllows(cfg.Custom, trimmed) {
		return "name `" + name + "` doesn't satisfy the configured custom regex", false
	}
	// Format check. `format: null` (FormatNull) accepts any name.
	if cfg.FormatNull {
		return "", true
	}
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
	case "strictCamelCase", "StrictCamelCase":
		return isCamelCase(s, true)
	case "PascalCase":
		return isPascalCase(s, false)
	case "strictPascalCase", "StrictPascalCase":
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
			walk(node.BindingElementName())
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

func (r *rule) variableModifiers(ctx *engine.Context, n *wrapperchecker.Node, idNode *wrapperchecker.Node) []string {
	var mods []string
	if isConstVariable(n) {
		mods = append(mods, "const")
	}
	if hasExportedAncestor(n) || r.isExportedViaBlock(ctx, idNode) {
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
	if r.isUnused(ctx, idNode) {
		mods = append(mods, "unused")
	}
	if init := n.VariableDeclarationInitializer(); init != nil && wrapperchecker.HasAsyncModifier(init) {
		mods = append(mods, "async")
	}
	return mods
}

// isUnused reports whether the binding represented by idNode has no
// in-source references besides its own declaration. Computed by walking
// the SourceFile once per file (cached) and counting symbol references.
func (r *rule) isUnused(ctx *engine.Context, idNode *wrapperchecker.Node) bool {
	if idNode == nil {
		return false
	}
	sym := ctx.Checker().SymbolOf(idNode)
	if sym == nil {
		return false
	}
	// Exported (or default-exported) bindings are by definition used —
	// any external consumer counts as a reference.
	if isExportedSymbol(idNode) {
		return false
	}
	usage := r.usageMapFor(ctx, idNode)
	return usage[sym.ID()] == 0
}

// isExportedViaBlock reports whether the name has a matching local
// binding referenced by an `export { ... }` declaration in the same
// source file. Cached per source file by name text.
func (r *rule) isExportedViaBlock(ctx *engine.Context, idNode *wrapperchecker.Node) bool {
	if idNode == nil {
		return false
	}
	name := idNode.LiteralText()
	if name == "" {
		return false
	}
	sf := containingSourceFile(idNode)
	if sf == nil {
		return false
	}
	key, _, _, _, _ := sf.SourceRange()
	if r.exportNameCache == nil {
		r.exportNameCache = map[string]map[string]struct{}{}
	}
	set, ok := r.exportNameCache[key]
	if !ok {
		set = map[string]struct{}{}
		var walk func(n *wrapperchecker.Node)
		walk = func(n *wrapperchecker.Node) {
			if n == nil {
				return
			}
			if n.Kind() == wrapperchecker.KindExportSpecifier {
				// The local-binding identifier is the first
				// Identifier child (PropertyName when aliased, Name
				// otherwise). Either way, the first identifier names
				// the local binding being re-exported.
				var first *wrapperchecker.Node
				n.ForEachChild(func(c *wrapperchecker.Node) bool {
					if first == nil && c.Kind() == wrapperchecker.KindIdentifier {
						first = c
					}
					return false
				})
				if first != nil {
					if t := first.LiteralText(); t != "" {
						set[t] = struct{}{}
					}
				}
			}
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c)
				return false
			})
		}
		walk(sf)
		r.exportNameCache[key] = set
	}
	_, exported := set[name]
	return exported
}

func containsMod(mods []string, want string) bool {
	for _, m := range mods {
		if m == want {
			return true
		}
	}
	return false
}

// isExportedSymbol reports whether the declaration to which idNode
// belongs is exported (or default-exported) at any enclosing level.
func isExportedSymbol(idNode *wrapperchecker.Node) bool {
	for p := idNode.Parent(); p != nil; p = p.Parent() {
		if p.HasExportModifier() || p.HasDefaultModifier() {
			return true
		}
		switch p.Kind() {
		case wrapperchecker.KindSourceFile:
			return false
		}
	}
	return false
}

// usageMapFor returns a cached map from symbol pointer-key to the
// number of NON-declaring identifier references in the SourceFile that
// contains node. The cache is keyed by source file pointer so each file
// is scanned exactly once per linter run.
func (r *rule) usageMapFor(ctx *engine.Context, node *wrapperchecker.Node) map[uintptr]int {
	sf := containingSourceFile(node)
	if sf == nil {
		return nil
	}
	// Use the source file's reported file path as cache key — wrapper
	// Node identity isn't stable across walks.
	key, _, _, _, _ := sf.SourceRange()
	if r.usageCache == nil {
		r.usageCache = map[string]map[uintptr]int{}
	}
	if m, ok := r.usageCache[key]; ok {
		return m
	}
	m := map[uintptr]int{}
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier && !isDeclarationName(n) {
			if sym := ctx.Checker().SymbolOf(n); sym != nil {
				m[sym.ID()]++
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(sf)
	r.usageCache[key] = m
	return m
}

func containingSourceFile(n *wrapperchecker.Node) *wrapperchecker.Node {
	for p := n; p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindSourceFile {
			return p
		}
	}
	return nil
}

// isDeclarationName reports whether an Identifier node is the name slot
// of a declaration (rather than a reference). Heuristic: parent is a
// declaration kind and the identifier is its first identifier child.
func isDeclarationName(id *wrapperchecker.Node) bool {
	p := id.Parent()
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindPropertySignature,
		wrapperchecker.KindMethodSignature,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindEnumMember,
		wrapperchecker.KindTypeParameter,
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment,
		wrapperchecker.KindBindingElement:
		// Identifier is a declaration name if it's the first identifier
		// child of the parent.
		var first *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil && c.Kind() == wrapperchecker.KindIdentifier {
				first = c
			}
			return false
		})
		return first != nil && first.Same(id)
	}
	return false
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
	id := nameNode(n)
	if id != nil && id.Kind() == wrapperchecker.KindPrivateIdentifier {
		mods = append(mods, "#private")
	}
	if nameRequiresQuotes(id) {
		mods = append(mods, "requiresQuotes")
	}
	return mods
}

// nameRequiresQuotes reports whether the given name node would have
// required quoting to write out (e.g. `'a a'` because of the space).
// Identifiers and numeric literals never require quotes; string
// literals do unless every char is a valid identifier char.
func nameRequiresQuotes(id *wrapperchecker.Node) bool {
	if id == nil || id.Kind() != wrapperchecker.KindStringLiteral {
		return false
	}
	s := id.LiteralText()
	if s == "" {
		return true
	}
	for i, r := range s {
		if i == 0 {
			if !isIDStart(r) {
				return true
			}
			continue
		}
		if !isIDCont(r) {
			return true
		}
	}
	return false
}

func isIDStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_' || r == '$'
}

func isIDCont(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$'
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
	return varDecl.IsConstVariableDeclaration()
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
