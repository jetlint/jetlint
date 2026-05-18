// Package useforof implements use-for-of: a classic
// `for (let i = 0; i < arr.length; i++)` that only uses `arr[i]` reads
// more directly as `for (const x of arr)`.
package useforof

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-for-of"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindForStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	indexName, arrName, body := matchClassicLoop(n)
	if indexName == "" {
		return
	}
	if !bodyUsesOnlyIndexedRead(body, indexName, arrName) {
		return
	}
	ctx.Report(n, "this `for(i; i<arr.length; i++)` loop reads more directly as `for (const x of arr)`")
}

// matchClassicLoop returns (indexName, arrName, body) when the for
// loop matches `for (let|var i = 0; i < arr.length; i++)` or the
// `i++` variant. Returns ("","",nil) otherwise.
func matchClassicLoop(n *wrapperchecker.Node) (string, string, *wrapperchecker.Node) {
	// Initializer: VariableDeclarationList with one declaration `i = 0`.
	var init, cond, incr, body *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch idx {
		case 0:
			init = c
		case 1:
			cond = c
		case 2:
			incr = c
		case 3:
			body = c
		}
		idx++
		return false
	})
	cond = n.ForStatementCondition()
	incr = n.ForStatementIncrementor()
	if init == nil || cond == nil || incr == nil || body == nil {
		return "", "", nil
	}
	if init.Kind() != wrapperchecker.KindVariableDeclarationList {
		return "", "", nil
	}
	var decls []*wrapperchecker.Node
	init.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindVariableDeclaration {
			decls = append(decls, c)
		}
		return false
	})
	if len(decls) != 1 {
		return "", "", nil
	}
	// Get declared name and initializer value.
	var dname, dinit *wrapperchecker.Node
	dIdx := 0
	decls[0].ForEachChild(func(c *wrapperchecker.Node) bool {
		switch dIdx {
		case 0:
			dname = c
		case 1:
			dinit = c
		}
		dIdx++
		return false
	})
	if dname == nil || dname.Kind() != wrapperchecker.KindIdentifier {
		return "", "", nil
	}
	if dinit == nil || dinit.Kind() != wrapperchecker.KindNumericLiteral || dinit.SourceText() != "0" {
		return "", "", nil
	}
	indexName := dname.SourceText()
	// Condition: `i < arr.length`.
	if cond.Kind() != wrapperchecker.KindBinaryExpression || cond.BinaryOperatorKind() != wrapperchecker.KindLessThanToken {
		return "", "", nil
	}
	cl := cond.BinaryLeft()
	cr := cond.BinaryRight()
	if cl == nil || cl.Kind() != wrapperchecker.KindIdentifier || cl.SourceText() != indexName {
		return "", "", nil
	}
	if cr == nil || cr.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return "", "", nil
	}
	arrName, prop := propParts(cr)
	if prop != "length" || arrName == nil {
		return "", "", nil
	}
	// Incrementor must be `i++` / `++i` / `i += 1` / `i = i + 1`.
	if !isIncrementOf(incr, indexName) {
		return "", "", nil
	}
	return indexName, arrName.SourceText(), body
}

func isIncrementOf(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindPostfixUnaryExpression, wrapperchecker.KindPrefixUnaryExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner == nil || inner.Kind() != wrapperchecker.KindIdentifier || inner.SourceText() != name {
			return false
		}
		src := n.SourceText()
		return strings.Contains(src, "++")
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		if op == wrapperchecker.KindPlusEqualsToken {
			left := n.BinaryLeft()
			right := n.BinaryRight()
			return left != nil && left.SourceText() == name && right != nil && right.SourceText() == "1"
		}
	}
	return false
}

func bodyUsesOnlyIndexedRead(body *wrapperchecker.Node, indexName, arrName string) bool {
	// Walk body. Any reference to indexName must be the index of an
	// ElementAccessExpression on arrName, AND not on the LHS of an
	// assignment / delete / destructuring.
	ok := true
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if !ok || n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier && n.SourceText() == indexName {
			// Must be inside `arr[i]` reading position.
			p := n.Parent()
			if p == nil || p.Kind() != wrapperchecker.KindElementAccessExpression {
				ok = false
				return
			}
			var obj, idxNode *wrapperchecker.Node
			pIdx := 0
			p.ForEachChild(func(c *wrapperchecker.Node) bool {
				switch pIdx {
				case 0:
					obj = c
				case 1:
					idxNode = c
				}
				pIdx++
				return false
			})
			if idxNode != n {
				ok = false
				return
			}
			if obj == nil || obj.Kind() != wrapperchecker.KindIdentifier || obj.SourceText() != arrName {
				ok = false
				return
			}
			// arr[i] must not be the LHS of an assignment.
			gp := p.Parent()
			if gp != nil && gp.Kind() == wrapperchecker.KindBinaryExpression {
				op := gp.BinaryOperatorKind()
				if op == wrapperchecker.KindEqualsToken || op == wrapperchecker.KindPlusEqualsToken ||
					op == wrapperchecker.KindMinusEqualsToken {
					if gp.BinaryLeft() == p {
						ok = false
						return
					}
				}
			}
			// arr[i] must not be a delete target.
			if gp != nil && gp.Kind() == wrapperchecker.KindDeleteExpression {
				ok = false
				return
			}
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return ok
}

func propParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return nil, ""
	}
	return first, second.SourceText()
}
