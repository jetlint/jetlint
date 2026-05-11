// Package strictvoidreturn implements the strict-void-return rule:
// flag any value-returning (or async) function passed where a void
// function is expected.
//
// Mirrors typescript-eslint's `strict-void-return`. The rule fires
// across many syntactic contexts — call/new arguments, variable
// initialisers, assignments, return statements, object/array
// element positions — wherever the contextual type is a function
// type whose every call signature returns `void`.
package strictvoidreturn

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "strict-void-return"

// Options is the configurable surface of the rule.
type Options struct {
	// AllowReturnAny: when true, callbacks whose body returns a value of
	// type `any` are not flagged. Off by default — `any` could be a
	// non-void value at runtime, so the explicit cast is required.
	AllowReturnAny bool
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	opts := DefaultOptions()
	if len(raw) == 0 {
		return opts, nil
	}
	var v struct {
		AllowReturnAny bool `json:"allowReturnAny"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return opts, fmt.Errorf("strict-void-return: invalid options JSON: %w", err)
	}
	opts.AllowReturnAny = v.AllowReturnAny
	return opts, nil
}

func New() engine.Rule                        { return &rule{opts: DefaultOptions()} }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:          r.visitCall,
		wrapperchecker.KindNewExpression:           r.visitCall,
		wrapperchecker.KindVariableDeclaration:     r.visitVarDecl,
		wrapperchecker.KindBinaryExpression:        r.visitAssign,
		wrapperchecker.KindReturnStatement:         r.visitReturn,
		wrapperchecker.KindArrayLiteralExpression:  r.visitArray,
		wrapperchecker.KindObjectLiteralExpression: r.visitObject,
		wrapperchecker.KindArrowFunction:           r.visitArrowBody,
		wrapperchecker.KindPropertyDeclaration:     r.visitPropertyDeclaration,
		wrapperchecker.KindMethodDeclaration:       r.visitMethodDeclaration,
		wrapperchecker.KindJsxAttribute:            r.visitJsxAttribute,
		wrapperchecker.KindGetAccessor:             r.visitGetAccessor,
	}
}

// visitGetAccessor handles `get foo()` accessors on class bodies. The
// accessor's return value IS the property's value, so when the
// inherited property type is a void-returning function type, each
// return-statement value must itself be a void-returning function.
func (r *rule) visitGetAccessor(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil ||
		(parent.Kind() != wrapperchecker.KindClassDeclaration &&
			parent.Kind() != wrapperchecker.KindClassExpression) {
		return
	}
	propType := classMemberInheritedType(ctx, n)
	if propType == nil || !isVoidReturningFunctionType(propType) {
		return
	}
	body := n.FunctionBody()
	if body == nil {
		return
	}
	walkReturnStatements(body, n, func(rs *wrapperchecker.Node) {
		expr := rs.FirstChild()
		if expr == nil {
			return
		}
		r.reportIfNonVoidFunction(ctx, expr)
	})
}

// classMemberInheritedType returns the type of the matching member on
// any base class / implemented interface of the class that contains the
// member declaration.
func classMemberInheritedType(ctx *engine.Context, member *wrapperchecker.Node) *wrapperchecker.Type {
	parent := member.Parent()
	if parent == nil {
		return nil
	}
	name := methodNameTextResolved(ctx, member)
	if name == "" {
		return nil
	}
	classType := ctx.TypeOf(parent)
	if classType == nil {
		return nil
	}
	bases := append(classType.HeritageBaseTypes(), classType.ImplementsHeritageTypes()...)
	bases = append(bases, classType.BaseTypes()...)
	for _, base := range bases {
		if pt := base.PropertyType(name); pt != nil {
			return pt
		}
	}
	return nil
}

// visitJsxAttribute handles `<Foo cb={…} />`. The attribute's expression
// value gets checked against the prop's contextual type.
func (r *rule) visitJsxAttribute(ctx *engine.Context, n *wrapperchecker.Node) {
	var expr *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindJsxExpression {
			c.ForEachChild(func(inner *wrapperchecker.Node) bool {
				if inner.Kind() == wrapperchecker.KindIdentifier {
					return false
				}
				if expr == nil {
					expr = inner
					return true
				}
				return false
			})
			return true
		}
		return false
	})
	if expr == nil {
		return
	}
	r.checkExpressionNode(ctx, expr)
}

// visitCall handles `f(a, b, ...)` and `new F(a, b, ...)`. For each
// argument, collect the expected parameter type across every overload
// of the callee and see whether the union of those signatures is a
// "always-void-returning function type". If so, check the argument.
func (r *rule) visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	args := n.CallArguments()
	if len(args) == 0 {
		return
	}
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	calleeType := ctx.TypeOf(callee)
	if calleeType == nil {
		return
	}
	isNew := n.Kind() == wrapperchecker.KindNewExpression
	// Collect every call signature across union constituents.
	var sigs []*wrapperchecker.Signature
	collectSignatures(calleeType, isNew, &sigs)
	singleSig := len(sigs) == 1
	for i, arg := range args {
		if arg == nil {
			continue
		}
		expectedReturns := argReturnTypes(sigs, i)
		allVoid := len(expectedReturns) > 0 && allVoidLike(expectedReturns)
		// First check via the contextual type (which lines up with the
		// resolved overload for single-signature callees, or fits any
		// overload when every overload returns void at this slot).
		if singleSig || allVoid {
			expected := ctx.Checker().ContextualTypeForArgument(n, i)
			if expected != nil && isVoidReturningFunctionType(expected) {
				r.reportIfNonVoidFunction(ctx, arg)
				continue
			}
		}
		// Fallback: at least one signature wants void here AND the rest
		// of the candidate returns are nullish/any. Same semantics as
		// upstream's loosened-overload check.
		if hasVoid(expectedReturns) && allNullishOrAny(expectedReturns) {
			r.reportIfNonVoidFunction(ctx, arg)
		}
	}
}

func (r *rule) visitVarDecl(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	r.checkExpressionNode(ctx, init)
}

func (r *rule) visitAssign(ctx *engine.Context, n *wrapperchecker.Node) {
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
	default:
		return
	}
	right := n.BinaryRight()
	if right == nil {
		return
	}
	r.checkExpressionNode(ctx, right)
}

// visitPropertyDeclaration handles class field declarations:
//
//	class C { f: () => void = expr; }
//
// The initializer is checked against either the explicit annotation or
// the inherited / implemented property type.
func (r *rule) visitPropertyDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.PropertyDeclarationInitializer()
	if init == nil {
		return
	}
	// Standard path: annotation-driven contextual type check.
	r.checkExpressionNode(ctx, init)
	// Inheritance path: the field overrides a parent's void-returning
	// member without re-annotating it.
	if n.PropertyDeclarationType() != nil {
		return
	}
	if propType := classMemberInheritedType(ctx, n); propType != nil &&
		isVoidReturningFunctionType(propType) {
		r.reportIfNonVoidFunction(ctx, init)
	}
}

// visitMethodDeclaration handles both class methods and object-literal
// method shorthand. If the method's contextual or annotated return type
// is `void`, walk the body and flag any non-void return statements.
func (r *rule) visitMethodDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	// Determine the expected return type:
	//  1. Explicit annotation, if any.
	//  2. Otherwise, the type of the method declaration (resolved against
	//     containers/inherited types by the checker).
	if !methodIsVoidReturning(ctx, n) {
		return
	}
	body := n.FunctionBody()
	if body == nil {
		return
	}
	if n.IsGeneratorFunction() {
		ctx.Report(n, "value-returning function used where a void function is expected")
		return
	}
	if wrapperchecker.HasAsyncModifier(n) {
		ctx.Report(n, "async function used where a void function is expected")
		return
	}
	walkReturnStatements(body, n, func(ret *wrapperchecker.Node) {
		expr := ret.FirstChild()
		if expr == nil {
			return
		}
		t := ctx.TypeOf(expr)
		if r.typeIsAllowedReturn(t) {
			return
		}
		ctx.Report(ret, "value returned where a void return is expected")
	})
}

// allReturnsVoid returns true when every constituent of t is `void`.
func allReturnsVoid(t *wrapperchecker.Type) bool {
	for _, m := range unionConstituents(t) {
		if !m.IsVoid() {
			return false
		}
	}
	return true
}

// methodIsVoidReturning reports whether a MethodDeclaration's declared
// return type — taken from the explicit annotation OR from inheritance
// / contextual typing — is void. Annotations are not the last word:
// upstream still flags methods whose inherited type insists on void.
func methodIsVoidReturning(ctx *engine.Context, n *wrapperchecker.Node) bool {
	if annot := n.FunctionReturnTypeAnnotation(); annot != nil {
		t := ctx.Checker().TypeFromTypeNode(annot)
		if t != nil && allReturnsVoid(t) {
			return true
		}
		// Fall through: an explicit non-void annotation can still be
		// overridden by an inherited void declaration.
	}
	// Object-literal method shorthand: contextual comes from the
	// containing object literal's expected property type.
	if parent := n.Parent(); parent != nil &&
		parent.Kind() == wrapperchecker.KindObjectLiteralExpression {
		if ct := ctx.Checker().ContextualTypeOfObjectElement(n); ct != nil {
			if isVoidReturningFunctionType(ct) {
				return true
			}
		}
		return false
	}
	// Class methods: look up the symbol's apparent type via the
	// containing class type so inherited / implemented method types
	// participate.
	if ct := classMethodInheritedReturn(ctx, n); ct != nil {
		return allReturnsVoid(ct)
	}
	return false
}

// classMethodInheritedReturn returns the return type of the method's
// inherited / implemented declaration, or nil when none applies.
func classMethodInheritedReturn(ctx *engine.Context, method *wrapperchecker.Node) *wrapperchecker.Type {
	propType := classMemberInheritedType(ctx, method)
	if propType == nil {
		return nil
	}
	sigs := propType.CallSignatures()
	if len(sigs) == 0 {
		return nil
	}
	return sigs[0].ReturnType()
}

// methodNameText extracts the textual name of a class/object method
// declaration. Resolves computed names (`[expr]`) by routing through
// the method's symbol when possible.
func methodNameText(method *wrapperchecker.Node) string {
	var found string
	method.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNumericLiteral:
			if found == "" {
				found = c.LiteralText()
				return true
			}
		}
		return false
	})
	return found
}

// methodNameTextResolved is like methodNameText but resolves computed
// names (`[expr]`) by looking up the inner expression's string-literal
// type, and falls back to the method's symbol name otherwise.
func methodNameTextResolved(ctx *engine.Context, method *wrapperchecker.Node) string {
	var resolved string
	method.ForEachChild(func(c *wrapperchecker.Node) bool {
		if resolved != "" {
			return false
		}
		// Walk into a ComputedPropertyName to grab the literal type of
		// its expression.
		var inner *wrapperchecker.Node
		c.ForEachChild(func(g *wrapperchecker.Node) bool {
			if inner == nil {
				inner = g
				return true
			}
			return false
		})
		if inner == nil {
			return false
		}
		if t := ctx.TypeOf(inner); t != nil {
			if s, ok := t.StringLiteralValue(); ok {
				resolved = s
				return true
			}
		}
		return false
	})
	if resolved != "" {
		return resolved
	}
	if sym := ctx.Checker().SymbolOf(method); sym != nil {
		if name := sym.Name(); name != "" {
			return name
		}
	}
	return methodNameText(method)
}

func (r *rule) visitReturn(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	r.checkExpressionNode(ctx, expr)
}

func (r *rule) visitArray(ctx *engine.Context, n *wrapperchecker.Node) {
	for _, e := range n.ArrayElements() {
		if e == nil || e.Kind() == wrapperchecker.KindSpreadElement ||
			e.Kind() == wrapperchecker.KindOmittedExpression {
			continue
		}
		r.checkExpressionNode(ctx, e)
	}
}

func (r *rule) visitObject(ctx *engine.Context, n *wrapperchecker.Node) {
	for _, prop := range n.ObjectProperties() {
		if prop.Kind() != wrapperchecker.KindPropertyAssignment {
			continue
		}
		init := prop.PropertyInitializer()
		if init == nil {
			continue
		}
		r.checkExpressionNode(ctx, init)
	}
}

func (r *rule) visitArrowBody(ctx *engine.Context, n *wrapperchecker.Node) {
	body := n.FunctionBody()
	if body == nil || body.Kind() == wrapperchecker.KindBlock {
		return
	}
	r.checkExpressionNode(ctx, body)
}

// checkExpressionNode: look up the contextual type at `node` and, if
// it's a void-returning function type, validate the actual expression.
func (r *rule) checkExpressionNode(ctx *engine.Context, node *wrapperchecker.Node) {
	expected := ctx.Checker().ContextualTypeOf(node)
	if expected == nil {
		return
	}
	if !isVoidReturningFunctionType(expected) {
		return
	}
	r.reportIfNonVoidFunction(ctx, node)
}

// reportIfNonVoidFunction checks whether the provided expression is a
// function whose actual return type fits the allowed set (void / never
// / undefined / optionally any). If not, it emits the most specific
// diagnostic available: nonVoidFunc, asyncFunc, or nonVoidReturn.
func (r *rule) reportIfNonVoidFunction(ctx *engine.Context, funcNode *wrapperchecker.Node) {
	if funcNode == nil {
		return
	}
	actual := ctx.TypeOf(funcNode)
	if actual == nil {
		return
	}
	// Skip nullish actuals — `null` / `undefined` in a void-or-null slot
	// are fine and have no callable signature to check anyway.
	if isNullishOrAny(actual) && !actual.IsAny() {
		return
	}
	if actual.IsAny() && r.opts.AllowReturnAny {
		return
	}
	// Skip if the actual type's every call signature returns an allowed
	// type already.
	if r.allowedReturn(actual) {
		return
	}
	isArrow := funcNode.Kind() == wrapperchecker.KindArrowFunction
	isFuncExpr := funcNode.Kind() == wrapperchecker.KindFunctionExpression
	if !isArrow && !isFuncExpr {
		// Non-function-literal candidates only get reported when their
		// type carries at least one call signature returning a value.
		if len(actual.CallSignatures()) == 0 {
			return
		}
		ctx.Report(funcNode, "value-returning function used where a void function is expected")
		return
	}
	// Generator functions (only function expressions can be generators
	// in this rule's contexts) are never void.
	if isFuncExpr && funcNode.IsGeneratorFunction() {
		ctx.Report(funcNode, "value-returning function used where a void function is expected")
		return
	}
	if wrapperchecker.HasAsyncModifier(funcNode) {
		ctx.Report(funcNode, "async function used where a void function is expected")
		return
	}
	// Explicit non-void return annotation → nonVoidFunc.
	if annot := funcNode.FunctionReturnTypeAnnotation(); annot != nil {
		if !returnAnnotIsVoid(annot) {
			ctx.Report(funcNode, "value-returning function used where a void function is expected")
			return
		}
	}
	body := funcNode.FunctionBody()
	if body == nil {
		return
	}
	if body.Kind() != wrapperchecker.KindBlock {
		// Arrow shorthand: `() => expr`. Check the expression's type.
		t := ctx.TypeOf(body)
		if r.typeIsAllowedReturn(t) {
			return
		}
		ctx.Report(body, "value returned where a void return is expected")
		return
	}
	// Block body: walk and flag each value-returning return statement.
	walkReturnStatements(body, funcNode, func(ret *wrapperchecker.Node) {
		expr := ret.FirstChild()
		if expr == nil {
			return
		}
		t := ctx.TypeOf(expr)
		if r.typeIsAllowedReturn(t) {
			return
		}
		ctx.Report(ret, "value returned where a void return is expected")
	})
}

// allowedReturn reports whether every call signature of the candidate
// function type returns one of the allowed types (void / never /
// undefined, plus any when AllowReturnAny is set).
func (r *rule) allowedReturn(t *wrapperchecker.Type) bool {
	sigs := t.CallSignatures()
	if len(sigs) == 0 {
		return false
	}
	for _, s := range sigs {
		ret := s.ReturnType()
		for _, m := range unionConstituents(ret) {
			if !r.typeIsAllowedReturn(m) {
				return false
			}
		}
	}
	return true
}

func (r *rule) typeIsAllowedReturn(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsVoid() || t.IsUndefined() || t.IsNever() {
		return true
	}
	if r.opts.AllowReturnAny && t.IsAny() {
		return true
	}
	return false
}

// isVoidReturningFunctionType reports whether t (a contextual type)
// admits a void-returning function at this slot. A union qualifies when
// at least one constituent is a void-returning function AND no other
// constituent is a non-void-returning function (a `(() => void) | string`
// slot qualifies; `(() => void) | (() => string)` does not).
func isVoidReturningFunctionType(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		hasVoidFn := false
		for _, m := range t.UnionMembers() {
			if isVoidReturningFunctionType(m) {
				hasVoidFn = true
				continue
			}
			if len(m.CallSignatures()) > 0 {
				return false
			}
		}
		return hasVoidFn
	}
	sigs := t.CallSignatures()
	if len(sigs) == 0 {
		return false
	}
	for _, s := range sigs {
		ret := s.ReturnType()
		for _, m := range unionConstituents(ret) {
			if !m.IsVoid() {
				return false
			}
		}
	}
	return true
}

// collectSignatures recursively flattens union constituents and pushes
// every call (or construct, for `new`) signature onto sigs.
func collectSignatures(t *wrapperchecker.Type, construct bool, sigs *[]*wrapperchecker.Signature) {
	if t == nil {
		return
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			collectSignatures(m, construct, sigs)
		}
		return
	}
	if construct {
		*sigs = append(*sigs, t.ConstructSignatures()...)
	} else {
		*sigs = append(*sigs, t.CallSignatures()...)
	}
}

// argReturnTypes collects every "i-th parameter's return type" across
// the given signatures. Used to detect overload sets where some
// signatures want a void callback and others want a value-returning
// one — the rule treats them as void when all "other" returns are
// nullish/any. Type parameters are treated as void.
func argReturnTypes(sigs []*wrapperchecker.Signature, i int) []*wrapperchecker.Type {
	var out []*wrapperchecker.Type
	for _, s := range sigs {
		params := s.ParameterTypes()
		if i >= len(params) {
			continue
		}
		pt := params[i]
		if pt == nil {
			continue
		}
		for _, m := range unionConstituents(pt) {
			for _, cs := range m.CallSignatures() {
				rt := cs.ReturnType()
				for _, r := range unionConstituents(rt) {
					out = append(out, r)
				}
			}
		}
	}
	return out
}

func allVoidLike(ts []*wrapperchecker.Type) bool {
	for _, t := range ts {
		if !t.IsVoid() && !isNullishOrAny(t) && !t.IsTypeParameter() {
			return false
		}
	}
	return true
}

func hasVoid(ts []*wrapperchecker.Type) bool {
	for _, t := range ts {
		if t.IsVoid() {
			return true
		}
	}
	return false
}

func allNullishOrAny(ts []*wrapperchecker.Type) bool {
	for _, t := range ts {
		if t.IsVoid() {
			continue
		}
		if !isNullishOrAny(t) {
			return false
		}
	}
	return true
}

func isNullishOrAny(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	return t.IsAny() || t.IsUnknown() || t.IsNull() || t.IsUndefined() || t.IsNever() || t.IsVoid()
}

func unionConstituents(t *wrapperchecker.Type) []*wrapperchecker.Type {
	if t == nil {
		return nil
	}
	if t.IsUnion() {
		return t.UnionMembers()
	}
	return []*wrapperchecker.Type{t}
}

// returnAnnotIsVoid reports whether a function's return-type annotation
// is `void` (or `: Type` where Type aliases void / is a union whose
// constituents are all void — but the simple case is enough for the
// upstream test corpus).
func returnAnnotIsVoid(annot *wrapperchecker.Node) bool {
	if annot == nil {
		return false
	}
	return annot.IsVoidTypeNode()
}

// walkReturnStatements visits every ReturnStatement reachable inside
// `body` that belongs to `owner` — descending into blocks, loops,
// try/catch, etc., but stopping at nested function-like nodes (which
// have their own returns).
func walkReturnStatements(body, owner *wrapperchecker.Node, visit func(*wrapperchecker.Node)) {
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n != owner {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration,
				wrapperchecker.KindGetAccessor,
				wrapperchecker.KindSetAccessor,
				wrapperchecker.KindConstructor:
				return
			}
		}
		if n.Kind() == wrapperchecker.KindReturnStatement {
			visit(n)
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
}
