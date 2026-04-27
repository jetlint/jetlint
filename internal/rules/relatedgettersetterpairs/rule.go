// Package relatedgettersetterpairs implements the related-getter-setter-pairs
// rule: flag a class/interface where a getter and setter share a name
// but have incompatible types (the setter's parameter type isn't a
// supertype of the getter's return type, so an assignment-then-read
// round trip can lose information).
package relatedgettersetterpairs

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "related-getter-setter-pairs"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindInterfaceDeclaration: visit,
		wrapperchecker.KindClassDeclaration:     visit,
		wrapperchecker.KindClassExpression:      visit,
		wrapperchecker.KindTypeLiteral:          visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	getters := map[string]*wrapperchecker.Node{}
	setters := map[string]*wrapperchecker.Node{}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindGetAccessor:
			if name := accessorName(c); name != "" {
				getters[name] = c
			}
		case wrapperchecker.KindSetAccessor:
			if name := accessorName(c); name != "" {
				setters[name] = c
			}
		}
		return false
	})
	for name, getter := range getters {
		setter := setters[name]
		if setter == nil {
			continue
		}
		gt := getterReturnType(ctx, getter)
		st := setterParamType(ctx, setter)
		if gt == nil || st == nil {
			continue
		}
		// Setter's parameter type must accept getter's return type.
		// `set value(x: string)` + `get value(): string | undefined` —
		// reading and writing back drops the undefined branch.
		if !gt.IsAssignableTo(st) {
			ctx.Report(getter, "getter and setter for '"+name+"' have incompatible types; round-tripping the value can lose information")
		}
	}
}

func accessorName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// getterReturnType returns the type the getter declares. For
// `get value(): T`, returns T.
func getterReturnType(ctx *engine.Context, n *wrapperchecker.Node) *wrapperchecker.Type {
	ann := n.FunctionReturnTypeAnnotation()
	if ann == nil {
		return nil
	}
	return ctx.Checker().TypeFromTypeNode(ann)
}

// setterParamType returns the parameter type of the setter
// (`set value(x: T)` returns T). Returns nil for empty parameter
// lists or malformed setters with more than one parameter (those
// aren't valid TypeScript setters and the rule shouldn't apply).
func setterParamType(ctx *engine.Context, n *wrapperchecker.Node) *wrapperchecker.Type {
	var params []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			params = append(params, c)
		}
		return false
	})
	if len(params) != 1 {
		return nil
	}
	var paramAnnotation *wrapperchecker.Node
	depth := 0
	params[0].ForEachChild(func(cc *wrapperchecker.Node) bool {
		if depth == 1 {
			paramAnnotation = cc
			return true
		}
		depth++
		return false
	})
	if paramAnnotation == nil {
		return nil
	}
	return ctx.Checker().TypeFromTypeNode(paramAnnotation)
}
