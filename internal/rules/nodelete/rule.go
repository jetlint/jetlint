// Package nodelete implements no-delete: flag `delete` expressions
// that target static object properties (`delete obj.foo`,
// `delete obj["foo"]`). The `delete` operator is slow in modern JS
// engines because it forces hidden-class transitions; setting the
// property to `undefined` (or omitting it via destructure / spread)
// is dramatically faster.
//
// Carve-outs match biome's behavior:
//   - `delete x` (bare identifier) — already a noop / SyntaxError in
//     strict mode, not what this rule is about.
//   - `delete f()` — the result of a call expression has no slot to
//     remove; engines fast-path it.
//   - `delete a[expr]` where expr is not a literal — dynamic property
//     access is sometimes the only option.
//   - `delete process.env.X` and `delete X.dataset.Y` — these talk to
//     host APIs that genuinely require `delete`.
package nodelete

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-delete"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindDeleteExpression: visit,
	}
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var out *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		out = c
		return true
	})
	return out
}

func visit(ctx *engine.Context, del *wrapperchecker.Node) {
	target := firstChild(del)
	if target == nil {
		return
	}
	if !isStaticMemberChain(target) {
		return
	}
	if startsWith(target, "process", "env") || endsWith(target, "dataset") {
		return
	}
	ctx.Report(del, "avoid `delete` on object properties — set to `undefined` or omit the key instead")
}

// isStaticMemberChain returns true if the target is a chain of
// property accesses (a.b.c, a["b"].c, a?.b?.["c"]) where every
// computed segment uses a literal index. A non-literal computed
// access (a[x]) or a non-member-access head (call/identifier)
// disqualifies the chain.
func isStaticMemberChain(n *wrapperchecker.Node) bool {
	depth := 0
	for {
		switch n.Kind() {
		case wrapperchecker.KindPropertyAccessExpression:
			depth++
			n = n.PropertyAccessReceiver()
		case wrapperchecker.KindElementAccessExpression:
			arg := n.ElementAccessIndex()
			if arg == nil || arg.Kind() != wrapperchecker.KindStringLiteral {
				return false
			}
			depth++
			n = n.ElementAccessReceiver()
		default:
			// At chain head: only identifiers/`this` count as a
			// real receiver, and we must have walked through at
			// least one property/element access on the way down.
			if depth == 0 {
				return false
			}
			return n.Kind() == wrapperchecker.KindIdentifier ||
				n.Kind() == wrapperchecker.KindThisKeyword
		}
		if n == nil {
			return false
		}
	}
}

// startsWith returns true if the property-access chain rooted at n
// starts with the given identifiers (outermost = last in chain).
// `startsWith(n, "process", "env")` matches `process.env.FOO`.
func startsWith(n *wrapperchecker.Node, head ...string) bool {
	// Walk to the chain head while collecting names in source order.
	names := chainNames(n)
	if len(names) < len(head) {
		return false
	}
	for i, h := range head {
		if names[i] != h {
			return false
		}
	}
	return true
}

// endsWith reports whether the chain's penultimate segment is name.
// For `a.b.c.dataset.x`, endsWith(..., "dataset") returns true.
func endsWith(n *wrapperchecker.Node, name string) bool {
	names := chainNames(n)
	if len(names) < 2 {
		return false
	}
	return names[len(names)-2] == name
}

// chainNames returns the property names in source order (outermost
// receiver first). String-literal index accesses contribute their
// literal value. Returns nil if the chain isn't pure.
func chainNames(n *wrapperchecker.Node) []string {
	var rev []string
	for {
		switch n.Kind() {
		case wrapperchecker.KindPropertyAccessExpression:
			rev = append(rev, n.PropertyAccessName())
			n = n.PropertyAccessReceiver()
		case wrapperchecker.KindElementAccessExpression:
			arg := n.ElementAccessIndex()
			if arg == nil || arg.Kind() != wrapperchecker.KindStringLiteral {
				return nil
			}
			rev = append(rev, arg.LiteralText())
			n = n.ElementAccessReceiver()
		case wrapperchecker.KindIdentifier:
			rev = append(rev, n.LiteralText())
			// Reverse to source order.
			for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
				rev[i], rev[j] = rev[j], rev[i]
			}
			return rev
		default:
			return nil
		}
		if n == nil {
			return nil
		}
	}
}
