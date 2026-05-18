// Package noempty implements the no-empty rule: empty block statements
// (`if (foo) {}`, `try {} catch (e) {}`) and empty switch statements
// almost always indicate refactoring that was never finished.
//
// Matches oxlint and ESLint's no-empty:
//   - Function/method/arrow bodies are out of scope — those belong to
//     no-empty-function.
//   - A block that contains only comments is treated as intentional
//     ("/* fall through */") and not reported.
//   - An empty switch always reports; a comment inside the braces does
//     not exempt it.
//   - The allowEmptyCatch option, when true, suppresses reports for
//     empty catch clauses but leaves empty try/finally blocks flagged.
package noempty

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-empty"

// Options configures the rule.
type Options struct {
	// AllowEmptyCatch suppresses reports for empty `catch` clauses.
	// Mirrors ESLint's allowEmptyCatch (default false).
	AllowEmptyCatch bool
}

// DefaultOptions returns the upstream default configuration.
func DefaultOptions() Options { return Options{} }

// OptionsFromJSON parses the rule's options object from raw JSON.
// Unknown keys are silently ignored, matching the lenient surface
// of most ESLint-style rules.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-empty options must be a JSON object: %w", err)
	}
	if v, ok := fields["allowEmptyCatch"]; ok {
		if err := json.Unmarshal(v, &out.AllowEmptyCatch); err != nil {
			return Options{}, fmt.Errorf("no-empty: allowEmptyCatch must be boolean: %w", err)
		}
	}
	return out, nil
}

// New constructs a noempty rule with default options.
func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

// NewWithOptions constructs a noempty rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBlock:           r.visitBlock,
		wrapperchecker.KindSwitchStatement: r.visitSwitch,
	}
}

func (r *rule) visitBlock(ctx *engine.Context, n *wrapperchecker.Node) {
	if len(n.BlockStatements()) > 0 {
		return
	}
	parent := n.Parent()
	if isFunctionLikeBody(parent) {
		return
	}
	if r.opts.AllowEmptyCatch && parent != nil &&
		parent.Kind() == wrapperchecker.KindCatchClause {
		return
	}
	if hasCommentInsideEmptyBlock(n.SourceText()) {
		return
	}
	ctx.Report(n, "Unexpected empty block statement")
}

func (r *rule) visitSwitch(ctx *engine.Context, n *wrapperchecker.Node) {
	if switchHasCases(n) {
		return
	}
	ctx.Report(n, "Unexpected empty switch statement")
}

// isFunctionLikeBody reports whether parent is a node whose Block
// child is the function/method body — those are out of scope for
// no-empty (handled by no-empty-function in upstream ESLint).
func isFunctionLikeBody(p *wrapperchecker.Node) bool {
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor:
		return true
	}
	return false
}

// switchHasCases reports whether the SwitchStatement contains any
// CaseClause or DefaultClause. The wrapper does not expose
// CaseBlock directly, so we walk one level down from the switch:
// its children are Expression and CaseBlock, and the CaseBlock's
// children are the clauses themselves.
func switchHasCases(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		c.ForEachChild(func(g *wrapperchecker.Node) bool {
			switch g.Kind() {
			case wrapperchecker.KindCaseClause, wrapperchecker.KindDefaultClause:
				found = true
				return true
			}
			return false
		})
		return found
	})
	return found
}

// hasCommentInsideEmptyBlock takes the literal source text of a block
// known to be empty (no statements) and reports whether it carries a
// comment. Since the block has no AST children, any non-whitespace
// character between the outer `{` and `}` must be a comment.
func hasCommentInsideEmptyBlock(src string) bool {
	open := strings.IndexByte(src, '{')
	closeIdx := strings.LastIndexByte(src, '}')
	if open < 0 || closeIdx < 0 || closeIdx <= open+1 {
		return false
	}
	for _, r := range src[open+1 : closeIdx] {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}
