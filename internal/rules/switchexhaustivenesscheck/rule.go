// Package switchexhaustivenesscheck implements the
// switch-exhaustiveness-check rule: flag switches over a union type
// that don't cover every member.
package switchexhaustivenesscheck

import (
	"regexp"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

// defaultCommentPattern matches typescript-eslint's default
// `defaultCaseCommentPattern` regex `/^no default$/iu`. A comment with
// trimmed text "no default" tells the rule the user opted out of a
// default clause intentionally and the switch should be treated as
// having one for exhaustiveness purposes.
var defaultCommentPattern = regexp.MustCompile(`(?i)^no default$`)

const id = "switch-exhaustiveness-check"

// Options is the configurable surface of the rule.
type Options struct {
	AllowDefaultCaseForExhaustiveSwitch bool
	RequireDefaultForNonUnion           bool
	ConsiderDefaultExhaustiveForUnions  bool
	// DefaultCaseCommentPattern overrides the regex used to detect a
	// "no default" comment that opts a switch out of the
	// missing-default report. Empty falls back to the upstream default
	// `^no default$` (case-insensitive).
	DefaultCaseCommentPattern string
}

// DefaultOptions matches typescript-eslint's defaults.
func DefaultOptions() Options {
	return Options{
		AllowDefaultCaseForExhaustiveSwitch: true,
	}
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule {
	r := &rule{opts: opts}
	if opts.DefaultCaseCommentPattern != "" {
		if re, err := regexp.Compile("(?i)" + opts.DefaultCaseCommentPattern); err == nil {
			r.commentPattern = re
		}
	}
	if r.commentPattern == nil {
		r.commentPattern = defaultCommentPattern
	}
	return r
}

type rule struct {
	opts           Options
	commentPattern *regexp.Regexp
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	disc := n.SwitchExpression()
	if disc == nil {
		return
	}
	dt := ctx.TypeOf(disc)
	if dt == nil {
		return
	}
	hasDefault, caseTypes := collectCases(ctx, n)
	if !hasDefault && hasDefaultCaseComment(n, r.commentPattern) {
		hasDefault = true
	}
	containsNonLiteral := containsNonLiteralType(dt)
	missing := missingLiteralBranches(dt, caseTypes)

	r.checkExhaustive(ctx, disc, missing, hasDefault)
	r.checkUnnecessaryDefault(ctx, disc, missing, hasDefault, containsNonLiteral)
	r.checkNoUnionDefault(ctx, disc, containsNonLiteral, hasDefault)
}

// checkExhaustive reports a single error when literal members of the
// discriminant union are missing case branches. Mirrors upstream's
// checkSwitchExhaustive: presence of a default clause suppresses the
// report only when considerDefaultExhaustiveForUnions is enabled.
func (r *rule) checkExhaustive(ctx *engine.Context, disc *wrapperchecker.Node, missing []string, hasDefault bool) {
	if r.opts.ConsiderDefaultExhaustiveForUnions && hasDefault {
		return
	}
	if len(missing) == 0 {
		return
	}
	ctx.Report(disc, "switch is not exhaustive — missing branches: "+joinBranches(missing))
}

// checkUnnecessaryDefault reports a default clause as superfluous on a
// fully-exhaustive switch over only literal-typed members.
// allowDefaultCaseForExhaustiveSwitch=false enables this guard.
func (r *rule) checkUnnecessaryDefault(ctx *engine.Context, disc *wrapperchecker.Node, missing []string, hasDefault, containsNonLiteral bool) {
	if r.opts.AllowDefaultCaseForExhaustiveSwitch {
		return
	}
	if hasDefault && len(missing) == 0 && !containsNonLiteral {
		ctx.Report(disc, "remove the `default` clause from this exhaustive switch")
	}
}

// checkNoUnionDefault reports a missing default clause when the
// discriminant has any non-literal-typed component (`number`,
// `string`, branded primitive, etc.). The option forces a default in
// that shape regardless of literal coverage.
func (r *rule) checkNoUnionDefault(ctx *engine.Context, disc *wrapperchecker.Node, containsNonLiteral, hasDefault bool) {
	if !r.opts.RequireDefaultForNonUnion {
		return
	}
	if containsNonLiteral && !hasDefault {
		ctx.Report(disc, "switch over a non-literal value should have a `default` clause")
	}
}

// joinBranches concatenates the missing branch labels into a single
// pipe-separated string, matching upstream's "{{missingBranches}}"
// data field shape.
func joinBranches(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " | "
		}
		out += p
	}
	return out
}

// missingLiteralBranches returns the literal-like union/intersection
// constituents of dt that aren't already covered by an explicit case
// clause. Mirrors upstream's missingLiteralBranchTypes computation.
func missingLiteralBranches(dt *wrapperchecker.Type, caseTypes map[string]bool) []string {
	var missing []string
	for _, unionPart := range unionConstituents(dt) {
		for _, intersectionPart := range intersectionConstituents(unionPart) {
			if !isLiteralLikeType(intersectionPart) {
				continue
			}
			label := intersectionPart.String()
			if caseTypes[label] {
				continue
			}
			missing = append(missing, label)
		}
	}
	return missing
}

// unionConstituents returns t's union members, or [t] for a non-union.
func unionConstituents(t *wrapperchecker.Type) []*wrapperchecker.Type {
	if t == nil {
		return nil
	}
	return t.UnionMembers()
}

// intersectionConstituents returns t's intersection members, or [t]
// for a non-intersection.
func intersectionConstituents(t *wrapperchecker.Type) []*wrapperchecker.Type {
	if t == nil {
		return nil
	}
	if t.IsIntersection() {
		return t.IntersectionMembers()
	}
	return []*wrapperchecker.Type{t}
}

// isLiteralLikeType reports whether t is a literal-typed flavor that
// can be cased — string/number/bigint literals, true/false, null,
// undefined, or unique symbol references.
func isLiteralLikeType(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsNullOrUndefined() {
		return true
	}
	if t.IsBooleanLike() {
		s := t.String()
		if s == "true" || s == "false" {
			return true
		}
	}
	s := t.String()
	if s == "" {
		return false
	}
	if t.IsStringLike() && s != "string" {
		return true
	}
	if t.IsNumberLike() && s != "number" {
		return true
	}
	if t.IsBigIntLike() && s != "bigint" {
		return true
	}
	if t.IsEnumLike() {
		return true
	}
	// Unique symbol references (`typeof a` for `const a = Symbol()`)
	// stringify to `typeof <name>` and are matchable by case clauses.
	if t.IsESSymbolLike() && s != "symbol" {
		return true
	}
	return false
}

// containsNonLiteralType reports whether the discriminant has any
// constituent (after intersection-flattening) whose every component
// is non-literal — i.e., a `string` / `number` / branded primitive
// where no concrete literal value is fixed. Mirrors upstream's
// doesTypeContainNonLiteralType.
func containsNonLiteralType(t *wrapperchecker.Type) bool {
	for _, unionPart := range unionConstituents(t) {
		nonLit := true
		for _, sub := range intersectionConstituents(unionPart) {
			if isLiteralLikeType(sub) {
				nonLit = false
				break
			}
		}
		if nonLit {
			return true
		}
	}
	return false
}

// hasDefaultCaseComment scans the switch's source for a single-line
// or block comment whose trimmed text matches the default-comment
// pattern. Used so `case 'a': break; // no default` is treated as
// having an explicit default for exhaustiveness purposes.
func hasDefaultCaseComment(sw *wrapperchecker.Node, pattern *regexp.Regexp) bool {
	src := sw.SourceText()
	if src == "" || pattern == nil {
		return false
	}
	for _, comment := range extractComments(src) {
		if pattern.MatchString(strings.TrimSpace(comment)) {
			return true
		}
	}
	return false
}

// extractComments returns the trimmed text of each `// ...` and `/* ... */`
// comment in src. Doesn't try to detect comments inside string or
// regex literals — switch bodies don't typically contain those at the
// top level, so misclassification is not a concern here.
func extractComments(src string) []string {
	var out []string
	i := 0
	for i < len(src) {
		if i+1 >= len(src) {
			break
		}
		if src[i] == '/' && src[i+1] == '/' {
			end := strings.Index(src[i:], "\n")
			if end == -1 {
				end = len(src) - i
			}
			out = append(out, src[i+2:i+end])
			i += end
			continue
		}
		if src[i] == '/' && src[i+1] == '*' {
			end := strings.Index(src[i+2:], "*/")
			if end == -1 {
				break
			}
			out = append(out, src[i+2:i+2+end])
			i += 2 + end + 2
			continue
		}
		i++
	}
	return out
}

// collectCases walks the switch body once, returning whether a default
// clause exists and the set of case-expression type strings already
// covered.
func collectCases(ctx *engine.Context, sw *wrapperchecker.Node) (bool, map[string]bool) {
	hasDefault := false
	covered := map[string]bool{}
	sw.ForEachChild(func(block *wrapperchecker.Node) bool {
		block.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindDefaultClause {
				hasDefault = true
				return false
			}
			if c.Kind() != wrapperchecker.KindCaseClause {
				return false
			}
			expr := c.CaseExpression()
			if expr == nil {
				return false
			}
			if t := ctx.TypeOf(expr); t != nil {
				covered[t.String()] = true
			}
			return false
		})
		return false
	})
	return hasDefault, covered
}
