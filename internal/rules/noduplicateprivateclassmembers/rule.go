// Package noduplicateprivateclassmembers implements
// no-duplicate-private-class-members: every private name in a class
// must be unique across all member kinds (field, method, getter,
// setter, accessor, static, instance — they all share one namespace).
// The single carveout is a paired getter+setter, which JavaScript
// treats as one accessor pair sharing the name.
package noduplicateprivateclassmembers

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-duplicate-private-class-members"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: visit,
		wrapperchecker.KindClassExpression:  visit,
	}
}

type memberSlot struct {
	getters int
	setters int
	others  int
	first   *wrapperchecker.Node
}

func visit(ctx *engine.Context, class *wrapperchecker.Node) {
	slots := map[string]*memberSlot{}
	class.ForEachChild(func(member *wrapperchecker.Node) bool {
		name, ok := privateMemberName(member)
		if !ok {
			return false
		}
		slot, present := slots[name]
		if !present {
			slot = &memberSlot{first: member}
			slots[name] = slot
		}
		switch member.Kind() {
		case wrapperchecker.KindGetAccessor:
			slot.getters++
		case wrapperchecker.KindSetAccessor:
			slot.setters++
		default:
			slot.others++
		}
		return false
	})
	for name, slot := range slots {
		if isDuplicate(slot) {
			ctx.Report(slot.first, "private member "+name+" is declared more than once in this class")
		}
	}
}

// isDuplicate reports whether a name slot has more than one
// declaration that doesn't form a valid getter/setter pair. A pair
// is one getter and one setter with no other declarations.
func isDuplicate(s *memberSlot) bool {
	if s.others > 1 {
		return true
	}
	if s.getters > 1 || s.setters > 1 {
		return true
	}
	if s.others == 1 && (s.getters > 0 || s.setters > 0) {
		return true
	}
	return false
}

// privateMemberName extracts the `#name` text from a class member
// whose name is a PrivateIdentifier. Returns ok=false for any
// member without a private name (regular fields, computed names,
// constructors, the class's own identifier).
func privateMemberName(member *wrapperchecker.Node) (string, bool) {
	switch member.Kind() {
	case wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
	default:
		return "", false
	}
	var name string
	member.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindPrivateIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	if name == "" {
		return "", false
	}
	return name, true
}
