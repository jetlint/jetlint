// Package nounexpectedmultiline implements the no-unexpected-multiline
// rule: in JavaScript, automatic semicolon insertion is *not* applied
// when the next line starts with a token that can continue the previous
// expression — an open paren, an open bracket, a backtick, or (with
// regex flag letters) a slash. The result is that
//
//	var a = b
//	(x || y).f()
//
// is parsed as `var a = b(x || y).f()`. This rule flags those four
// patterns so the developer can add an explicit semicolon.
package nounexpectedmultiline

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unexpected-multiline"

// New constructs a nounexpectedmultiline rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:           visitCall,
		wrapperchecker.KindElementAccessExpression:  visitElementAccess,
		wrapperchecker.KindTaggedTemplateExpression: visitTaggedTemplate,
		wrapperchecker.KindBinaryExpression:         visitBinary,
	}
}

// visitBinary handles the regex/division ambiguity: `foo\n/bar/gym` is
// parsed as `(foo / bar) / gym`, two divisions, when the programmer
// likely meant a regex literal. We detect the pattern by inspecting the
// raw source text of the outer division's right operand — if it looks
// like `[gimsuy]+` immediately following the slash and the inner
// division's slash sits on a new line from its left operand, flag.
func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isDivisionBinary(n) {
		return
	}
	parent := n.Parent()
	if parent == nil || parent.Kind() != wrapperchecker.KindBinaryExpression {
		return
	}
	if !isDivisionBinary(parent) {
		return
	}
	// Only handle the inner `a / b` whose result is the outer's left.
	pLeft := parent.BinaryLeft()
	if pLeft == nil || pLeft.Pos() != n.Pos() || pLeft.End() != n.End() {
		return
	}
	src := parent.SourceText()
	first := strings.IndexByte(src, '/')
	second := strings.LastIndex(src, "/")
	if first < 0 || second < 0 || first == second {
		return
	}
	if !strings.Contains(src[:first], "\n") {
		return
	}
	pRight := parent.BinaryRight()
	if pRight == nil {
		return
	}
	rightStartRel := pRight.Pos() - tokenPos(parent)
	if rightStartRel < 0 || rightStartRel >= len(src) {
		return
	}
	// The trailing identifier must look like regex flags and must sit
	// immediately after the second slash.
	tail := src[second+1:]
	idEnd := 0
	for idEnd < len(tail) {
		c := tail[idEnd]
		if c == '_' || c == '$' || (c >= '0' && c <= '9') ||
			(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			idEnd++
			continue
		}
		break
	}
	if idEnd == 0 {
		return
	}
	flags := tail[:idEnd]
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case 'g', 'i', 'm', 's', 'u', 'y':
		default:
			return
		}
	}
	if second+1 != rightStartRel {
		return
	}
	ctx.Report(parent, "Unexpected newline between numerator and division operator.")
}

// isDivisionBinary reports whether `n` is a BinaryExpression whose
// operator is `/`. The wrapper doesn't expose KindSlashToken, so we
// inspect the source text between the operands instead.
func isDivisionBinary(n *wrapperchecker.Node) bool {
	if n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return false
	}
	src := n.SourceText()
	leftEndRel := left.End() - tokenPos(n)
	rightStartRel := right.Pos() - tokenPos(n)
	if leftEndRel < 0 || rightStartRel < 0 || leftEndRel >= rightStartRel || rightStartRel > len(src) {
		return false
	}
	between := strings.TrimSpace(src[leftEndRel:rightStartRel])
	return between == "/"
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	// Optional-chain calls (`x?.()`) don't have the ASI ambiguity:
	// `\n?.()` cannot be parsed as a property access without `?.`.
	if isOptionalChainAccess(n) {
		return
	}
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	startRel := afterCalleeRel(n, callee)
	if startRel < 0 {
		return
	}
	checkBetween(ctx, n, startRel, '(',
		"Unexpected newline between function name and open parenthesis of function call.")
}

func visitElementAccess(ctx *engine.Context, n *wrapperchecker.Node) {
	if isOptionalChainAccess(n) {
		return
	}
	// First child is the object/receiver.
	var object *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		object = c
		return true
	})
	if object == nil {
		return
	}
	startRel := afterNodeRel(n, object)
	if startRel < 0 {
		return
	}
	checkBetween(ctx, n, startRel, '[',
		"Unexpected newline between object and open bracket of property access.")
}

func visitTaggedTemplate(ctx *engine.Context, n *wrapperchecker.Node) {
	// First child is the tag expression; type-argument list, if any,
	// comes between the tag and the template literal.
	var tag *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		tag = c
		return true
	})
	if tag == nil {
		return
	}
	startRel := afterTagRel(n, tag)
	if startRel < 0 {
		return
	}
	checkBetween(ctx, n, startRel, '`',
		"Unexpected newline between template tag and template literal.")
}

// checkBetween scans `n.SourceText()[startRel:]` for `target`. If a
// newline appears in the slice before the first occurrence of `target`,
// reports `msg`.
func checkBetween(ctx *engine.Context, n *wrapperchecker.Node, startRel int, target byte, msg string) {
	src := n.SourceText()
	if startRel < 0 || startRel > len(src) {
		return
	}
	tail := src[startRel:]
	t := strings.IndexByte(tail, target)
	if t < 0 {
		return
	}
	nl := strings.IndexByte(tail[:t], '\n')
	if nl < 0 {
		return
	}
	ctx.Report(n, msg)
}

// afterCalleeRel returns the position immediately after the callee
// (and any type-argument list) relative to the start of `call`'s source
// text. Returns -1 if the position can't be resolved.
func afterCalleeRel(call, callee *wrapperchecker.Node) int {
	end := callee.End()
	if args := call.TypeArgumentNodes(); len(args) > 0 {
		last := args[len(args)-1]
		if last.End() > end {
			end = last.End()
		}
		// Skip past the closing `>` after the last type arg.
		src := call.SourceText()
		rel := end - tokenPos(call)
		if rel >= 0 && rel < len(src) {
			if gt := strings.IndexByte(src[rel:], '>'); gt >= 0 {
				return rel + gt + 1
			}
		}
		return rel
	}
	return end - tokenPos(call)
}

// afterTagRel: same idea, but TaggedTemplateExpression's type args (if
// any) are TypeArgumentNodes on the tagged template itself.
func afterTagRel(tagged, tag *wrapperchecker.Node) int {
	end := tag.End()
	if args := tagged.TypeArgumentNodes(); len(args) > 0 {
		last := args[len(args)-1]
		if last.End() > end {
			end = last.End()
		}
		src := tagged.SourceText()
		rel := end - tokenPos(tagged)
		if rel >= 0 && rel < len(src) {
			if gt := strings.IndexByte(src[rel:], '>'); gt >= 0 {
				return rel + gt + 1
			}
		}
		return rel
	}
	return end - tokenPos(tagged)
}

func afterNodeRel(parent, child *wrapperchecker.Node) int {
	return child.End() - tokenPos(parent)
}

// tokenPos returns the position of `n`'s first token in the source —
// equivalent to oxc's `n.span().start`. We derive it from the SourceText
// length: SourceText returns text[startPos:end].
func tokenPos(n *wrapperchecker.Node) int {
	return n.End() - len(n.SourceText())
}

// isOptionalChainAccess reports whether a CallExpression or
// ElementAccessExpression is part of an optional chain (`a?.()`,
// `a?.[i]`). TS-go marks these with a NodeFlags bit.
func isOptionalChainAccess(n *wrapperchecker.Node) bool {
	// The wrapper exposes IsOptionalChain via parent navigation in
	// some rules; here we inspect the source-text snippet between the
	// receiver and the bracket/paren for `?.` to avoid depending on
	// a bit-flag accessor.
	src := n.SourceText()
	// Drop everything before the open-bracket/paren we care about by
	// scanning for `?.` anywhere in the immediate node. This is a
	// coarse but safe heuristic — overshooting only causes a missed
	// diagnostic, not a false positive.
	return strings.Contains(src, "?.")
}
