// Package nofallthrough implements the no-fallthrough rule: a
// `switch` case that contains statements but does not exit (via
// break / return / throw / continue or an equivalent always-exits
// construct) silently falls into the next case at runtime. We walk
// each switch's case clauses, ask whether the previous one always
// exits, and — if it doesn't — look for an opt-in fallthrough
// comment between the two cases.
package nofallthrough

import (
	"regexp"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-fallthrough"

// Options configures the rule.
type Options struct {
	// CommentPattern overrides the default fallthrough-comment
	// matcher. When set, the regex is wrapped with `(?i)` and used
	// instead of the four exact-string comparisons.
	CommentPattern string
	// AllowEmptyCase allows empty case clauses to fall through
	// regardless of intervening blank lines.
	AllowEmptyCase bool
	// ReportUnusedFallthroughComment, when true, reports a
	// fallthrough-comment that sits on a case whose predecessor
	// already exits explicitly (so the comment is misleading).
	ReportUnusedFallthroughComment bool
}

// DefaultOptions returns ESLint's defaults.
func DefaultOptions() Options { return Options{} }

// New constructs the rule with default options.
func New() engine.Rule { return &rule{opts: DefaultOptions()} }

// NewWithOptions constructs the rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule {
	r := &rule{opts: opts}
	if opts.CommentPattern != "" {
		r.commentRe = regexp.MustCompile("(?i)" + opts.CommentPattern)
	}
	return r
}

type rule struct {
	opts      Options
	commentRe *regexp.Regexp
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	cases := caseClauses(n)
	if len(cases) < 2 {
		return
	}
	for i := 0; i+1 < len(cases); i++ {
		cur := cases[i]
		next := cases[i+1]
		stmts := clauseStatements(cur)
		exits := caseExits(stmts)
		// Region where a fallthrough comment may appear:
		//  - When `cur` is exactly one block statement, oxc first
		//    checks the LAST comment INSIDE that block; if one
		//    matches, fallthrough is OK.
		//  - Otherwise (or as a fallback), it checks the LAST
		//    comment in the area between `cur`'s body end and the
		//    next case keyword.
		hasComment := false
		if len(stmts) == 1 && stmts[0].Kind() == wrapperchecker.KindBlock {
			if r.commentMatches(trailingTriviaInBlock(stmts[0])) {
				hasComment = true
			}
		}
		if !hasComment {
			hasComment = r.commentMatches(next.LeadingTriviaText())
		}
		commentRegion := next.LeadingTriviaText()

		if exits {
			if hasComment && r.opts.ReportUnusedFallthroughComment {
				ctx.Report(next, "Found a comment that would permit fallthrough, but case cannot fall through.")
			}
			continue
		}
		// Case ends without exit. If it has no statements at all,
		// allow unless allowEmptyCase=false AND there is a blank
		// line between cur and next (matches ESLint's behavior).
		if len(stmts) == 0 {
			if r.opts.AllowEmptyCase || !hasBlankLineInText(commentRegion) {
				continue
			}
		}
		if hasComment {
			continue
		}
		msg := "Expected a `break` statement before `case`."
		if next.Kind() == wrapperchecker.KindDefaultClause {
			msg = "Expected a `break` statement before `default`."
		}
		ctx.Report(next, msg)
	}
}

// trailingTriviaInBlock returns the source-text region inside a
// block, after its last contained statement (or the entire block
// content when the block is empty). That is the position where a
// fallthrough comment may sit when the case body is a single block.
func trailingTriviaInBlock(block *wrapperchecker.Node) string {
	body := block.BlockStatements()
	src := block.SourceText()
	if len(body) == 0 {
		return src
	}
	rel := body[len(body)-1].End() - block.Pos()
	if rel < 0 || rel > len(src) {
		return ""
	}
	return src[rel:]
}

// bodyAlwaysExitsCase reports whether a loop body always transfers
// control out of the enclosing case. A bare `break` inside the body
// targets the loop, not the case, so we walk skipping pure break /
// continue as terminators here.
func bodyAlwaysExitsCase(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindReturnStatement, wrapperchecker.KindThrowStatement:
		return true
	case wrapperchecker.KindBreakStatement, wrapperchecker.KindContinueStatement:
		return false
	case wrapperchecker.KindBlock:
		for _, s := range n.BlockStatements() {
			if bodyAlwaysExitsCase(s) {
				return true
			}
		}
		return false
	case wrapperchecker.KindIfStatement:
		then := n.IfThen()
		els := n.IfElse()
		if els == nil {
			return false
		}
		return bodyAlwaysExitsCase(then) && bodyAlwaysExitsCase(els)
	}
	return false
}

// caseExits reports whether the statement list — the body of a case
// clause — always exits in a way that the switch's next case is
// unreachable from it (i.e. break / return / throw / continue at
// the case's top level, or any compound construct that always
// exits).
func caseExits(stmts []*wrapperchecker.Node) bool {
	for _, s := range stmts {
		if statementAlwaysExits(s) {
			return true
		}
	}
	return false
}

func (r *rule) commentMatches(text string) bool {
	comments := extractComments(text)
	if len(comments) == 0 {
		return false
	}
	// The LAST comment immediately before the next case is the one
	// that opts into fallthrough — preceding comments don't count
	// once a later, non-matching comment exists.
	last := strings.TrimSpace(comments[len(comments)-1])
	if last == "" {
		return false
	}
	// `// eslint-disable-next-line no-fallthrough` directives are
	// honored regardless of pattern.
	low := strings.ToLower(last)
	if strings.Contains(low, "eslint-disable") && strings.Contains(low, "no-fallthrough") {
		return true
	}
	if r.commentRe != nil {
		return r.commentRe.MatchString(last)
	}
	return low == "falls through" || low == "fall through" || low == "fallsthrough" || low == "fallthrough"
}

// extractComments returns the inner text of each `//` and `/* */`
// comment found in `text`, in source order.
func extractComments(text string) []string {
	var out []string
	i := 0
	for i < len(text) {
		c := text[i]
		switch {
		case i+1 < len(text) && c == '/' && text[i+1] == '/':
			j := i + 2
			for j < len(text) && text[j] != '\n' && text[j] != '\r' {
				j++
			}
			out = append(out, text[i+2:j])
			i = j
		case i+1 < len(text) && c == '/' && text[i+1] == '*':
			j := i + 2
			for j+1 < len(text) && !(text[j] == '*' && text[j+1] == '/') {
				j++
			}
			if j+1 < len(text) {
				out = append(out, text[i+2:j])
				i = j + 2
			} else {
				out = append(out, text[i+2:])
				i = len(text)
			}
		default:
			i++
		}
	}
	return out
}

func hasBlankLineInText(text string) bool {
	// Strip comments so blank lines inside comments don't count.
	plain := stripBlockAndLineComments(text)
	// A blank line is two consecutive line terminators with only
	// whitespace between them.
	newlines := 0
	for _, r := range plain {
		switch r {
		case '\n':
			newlines++
			if newlines >= 2 {
				return true
			}
		case ' ', '\t', '\r':
		default:
			newlines = 0
		}
	}
	return false
}

// stripBlockAndLineComments removes comment contents but preserves
// surrounding whitespace, so blank-line detection considers code
// gaps but not comment-only lines.
func stripBlockAndLineComments(text string) string {
	var b strings.Builder
	i := 0
	for i < len(text) {
		c := text[i]
		switch {
		case i+1 < len(text) && c == '/' && text[i+1] == '/':
			j := i + 2
			for j < len(text) && text[j] != '\n' && text[j] != '\r' {
				j++
			}
			i = j
		case i+1 < len(text) && c == '/' && text[i+1] == '*':
			j := i + 2
			for j+1 < len(text) && !(text[j] == '*' && text[j+1] == '/') {
				j++
			}
			if j+1 < len(text) {
				i = j + 2
			} else {
				i = len(text)
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// caseClauses returns the case/default clauses of a switch. The
// wrapper does not export `KindCaseBlock`, so we descend two levels
// looking for clause children.
func caseClauses(switchStmt *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	switchStmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		c.ForEachChild(func(gc *wrapperchecker.Node) bool {
			if gc.Kind() == wrapperchecker.KindCaseClause || gc.Kind() == wrapperchecker.KindDefaultClause {
				out = append(out, gc)
			}
			return false
		})
		return false
	})
	return out
}

// clauseStatements returns the statement children of a case/default
// clause (excluding the case expression).
func clauseStatements(clause *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	clause.ForEachChild(func(c *wrapperchecker.Node) bool {
		if isStatementKind(c.Kind()) {
			out = append(out, c)
		}
		return false
	})
	return out
}

// kindEmptyStatement matches `;`. The wrapper does not export this
// constant; the value comes from internal/ast/kind_generated.go.
const kindEmptyStatement = wrapperchecker.Kind(243)

func isStatementKind(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindBlock,
		kindEmptyStatement,
		wrapperchecker.KindVariableStatement,
		wrapperchecker.KindExpressionStatement,
		wrapperchecker.KindIfStatement,
		wrapperchecker.KindDoStatement,
		wrapperchecker.KindWhileStatement,
		wrapperchecker.KindForStatement,
		wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement,
		wrapperchecker.KindContinueStatement,
		wrapperchecker.KindBreakStatement,
		wrapperchecker.KindReturnStatement,
		wrapperchecker.KindSwitchStatement,
		wrapperchecker.KindLabeledStatement,
		wrapperchecker.KindThrowStatement,
		wrapperchecker.KindTryStatement,
		wrapperchecker.KindDebuggerStatement,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration:
		return true
	}
	return false
}

func statementAlwaysExits(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindReturnStatement,
		wrapperchecker.KindThrowStatement,
		wrapperchecker.KindBreakStatement,
		wrapperchecker.KindContinueStatement:
		return true
	case wrapperchecker.KindBlock:
		for _, s := range n.BlockStatements() {
			if statementAlwaysExits(s) {
				return true
			}
		}
		return false
	case wrapperchecker.KindIfStatement:
		then := n.IfThen()
		els := n.IfElse()
		if els == nil {
			return false
		}
		return statementAlwaysExits(then) && statementAlwaysExits(els)
	case wrapperchecker.KindTryStatement:
		tryBlock := n.TryStatementTryBlock()
		catch := n.TryStatementCatchClause()
		finally := n.TryStatementFinallyBlock()
		if finally != nil && statementAlwaysExits(finally) {
			return true
		}
		tryExits := tryBlock != nil && statementAlwaysExits(tryBlock)
		if catch == nil {
			return tryExits
		}
		var catchBody *wrapperchecker.Node
		catch.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindBlock {
				catchBody = c
			}
			return false
		})
		catchExits := catchBody != nil && statementAlwaysExits(catchBody)
		return tryExits && catchExits
	case wrapperchecker.KindDoStatement:
		// A break inside the loop body breaks the loop, not the
		// enclosing case; so the loop body's "always exits" status
		// only propagates if it exits via return/throw/continue —
		// not via break.
		if body := n.IterationBody(); body != nil && bodyAlwaysExitsCase(body) {
			return true
		}
		return false
	case wrapperchecker.KindLabeledStatement:
		return false
	case wrapperchecker.KindSwitchStatement:
		return false
	}
	return false
}
