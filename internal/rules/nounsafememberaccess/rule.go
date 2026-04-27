// Package nounsafememberaccess implements the no-unsafe-member-access rule:
// flag `x.foo` or `x[key]` where x has type any.
package nounsafememberaccess

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-member-access"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: visit,
		wrapperchecker.KindElementAccessExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip access expressions inside heritage clauses (`extends X.Y`,
	// `implements X.Y`) — these are type-position references, not
	// runtime member accesses.
	if isInHeritagePosition(n) {
		return
	}
	// Only flag the outermost access in a chain so `x.a.b.c.d` (where
	// x is any) reports once instead of four times. We're the
	// outermost when our parent isn't a property/element access whose
	// own receiver is us.
	if isInnerAccessOfChain(n) {
		return
	}
	var recv *wrapperchecker.Node
	if n.Kind() == wrapperchecker.KindPropertyAccessExpression {
		recv = n.PropertyAccessReceiver()
	} else {
		recv = n.ElementAccessReceiver()
	}
	if recv == nil {
		return
	}
	rT := ctx.TypeOf(recv)
	if rT == nil {
		return
	}
	if rT.IsAny() {
		ctx.Report(n, "member access on a value of type `any` defeats the type checker")
		return
	}
	if n.Kind() == wrapperchecker.KindElementAccessExpression {
		idx := n.ElementAccessIndex()
		if idx == nil {
			return
		}
		if idxT := ctx.TypeOf(idx); idxT != nil && idxT.IsAny() {
			ctx.Report(n, "indexing with a value of type `any` defeats the type checker")
		}
	}
}

func isInHeritagePosition(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindPropertyAccessExpression,
			wrapperchecker.KindElementAccessExpression:
			continue
		case wrapperchecker.KindExpressionWithTypeArguments,
			wrapperchecker.KindHeritageClause:
			return true
		default:
			return false
		}
	}
	return false
}

func isInnerAccessOfChain(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	var pRecv *wrapperchecker.Node
	switch p.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		pRecv = p.PropertyAccessReceiver()
	case wrapperchecker.KindElementAccessExpression:
		pRecv = p.ElementAccessReceiver()
	default:
		return false
	}
	if pRecv == nil {
		return false
	}
	return sameNodeIdentity(pRecv, n)
}

func sameNodeIdentity(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	af, asl, asc, ael, aec := a.SourceRange()
	bf, bsl, bsc, bel, bec := b.SourceRange()
	return af == bf && asl == bsl && asc == bsc && ael == bel && aec == bec
}
