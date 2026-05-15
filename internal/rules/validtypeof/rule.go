// Package validtypeof implements the valid-typeof rule: flag
// `typeof x === "stirng"` and similar typos where the string operand
// is not one of the eight values JavaScript's typeof can actually
// return. The literal-equality framing covers `==`, `!=`, `===`, `!==`;
// relational operators against typeof are out of scope (they are
// nonsensical but produce a runtime error rather than a silent bug,
// so they belong to a different rule).
//
// String operands accept both regular string literals and template
// literals without expressions. When the non-typeof side is anything
// other than a constant string (a variable, a function call), the rule
// stays silent — the analysis is purely syntactic and refuses to guess.
package validtypeof

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "valid-typeof"

// Options is the configurable surface of valid-typeof.
type Options struct {
	// RequireStringLiterals tightens the rule: the non-typeof side
	// must be a string literal (or template literal with no
	// substitutions). With it off (the default), bare `undefined` is
	// still flagged because comparing a `typeof` result to the
	// undefined value never matches.
	RequireStringLiterals bool
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("valid-typeof options must be a JSON object: %w", err)
	}
	if v, ok := fields["requireStringLiterals"]; ok {
		if err := json.Unmarshal(v, &out.RequireStringLiterals); err != nil {
			return Options{}, err
		}
	}
	return out, nil
}

// New constructs a validtypeof rule instance with default options.
func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

// NewWithOptions constructs a validtypeof rule with the given options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

// validTypeofResults is the closed set of strings `typeof` may return
// at runtime. Comparing against any other literal is a typo.
var validTypeofResults = map[string]bool{
	"undefined": true,
	"object":    true,
	"boolean":   true,
	"number":    true,
	"string":    true,
	"function":  true,
	"symbol":    true,
	"bigint":    true,
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isEqualityOperator(n.BinaryOperatorKind()) {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	r.checkSide(ctx, left, right)
	r.checkSide(ctx, right, left)
}

// checkSide reports a diagnostic when typeofSide is `typeof X` and
// the other side is an invalid comparison value.
func (r *rule) checkSide(ctx *engine.Context, typeofSide, otherSide *wrapperchecker.Node) {
	if !isTypeofExpression(typeofSide) {
		return
	}
	if lit, ok := stringLiteralValue(otherSide); ok {
		if !validTypeofResults[lit] {
			ctx.Report(otherSide, "invalid typeof comparison value: "+lit+" is not a typeof result")
		}
		return
	}
	if isGlobalUndefined(ctx, otherSide) {
		ctx.Report(otherSide, "use \"undefined\" instead of undefined")
		return
	}
	if r.opts.RequireStringLiterals && !isTypeofExpression(otherSide) {
		ctx.Report(otherSide, "typeof comparisons should be against string literals")
	}
}

// isGlobalUndefined reports whether n is the bare identifier
// `undefined` that resolves to the global undefined value (not a
// local parameter or variable named "undefined"). Comparing a typeof
// result to the global undefined never matches, so it is flagged
// even without requireStringLiterals; a shadowed `undefined`
// parameter is just a misleadingly-named local and is left alone.
func isGlobalUndefined(ctx *engine.Context, n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	if n.LiteralText() != "undefined" {
		return false
	}
	sym := ctx.Checker().SymbolOf(n)
	if sym == nil {
		return true
	}
	for _, decl := range sym.Declarations() {
		if decl != nil && !isGlobalUndefinedDecl(decl) {
			return false
		}
	}
	return true
}

// isGlobalUndefinedDecl reports whether decl is part of TypeScript's
// built-in declaration of the global `undefined` (lib.es5.d.ts etc).
// User-supplied declarations (parameters, locals) make this false,
// so the rule treats `undefined` as a local shadow.
func isGlobalUndefinedDecl(decl *wrapperchecker.Node) bool {
	switch decl.Kind() {
	case wrapperchecker.KindParameter,
		wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration:
		return false
	}
	return true
}

func isEqualityOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken:
		return true
	}
	return false
}

func isTypeofExpression(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	return n.Kind() == wrapperchecker.KindTypeOfExpression
}

// stringLiteralValue returns the value of n if n is a plain string
// literal or a template literal without substitutions. The second
// return is false when n is anything else (a variable, an expression,
// a template with `${}` interpolation).
func stringLiteralValue(n *wrapperchecker.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText(), true
	}
	return "", false
}
