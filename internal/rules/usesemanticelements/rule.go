// Package usesemanticelements implements use-semantic-elements: when
// an HTML element already exists for a given ARIA role (e.g. <button>
// for role="button"), reach for the semantic tag rather than a div
// with a role attached. Native elements come with focus, keyboard
// handling, and form participation for free.
package usesemanticelements

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-semantic-elements"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// Roles whose semantic-HTML equivalent we expect authors to use.
// Anything not in this map is left alone (e.g. role="alert" has no
// single-tag equivalent and is fine on a div).
var semanticFor = map[string]string{
	"button":       "<button>",
	"checkbox":     `<input type="checkbox">`,
	"radio":        `<input type="radio">`,
	"searchbox":    `<input type="search">`,
	"textbox":      `<input type="text">`,
	"heading":      "<h1>-<h6>",
	"article":      "<article>",
	"figure":       "<figure>",
	"form":         "<form>",
	"navigation":   "<nav>",
	"main":         "<main>",
	"region":       "<section>",
	"complementary": "<aside>",
	"contentinfo":  "<footer>",
	"search":       "<search>",
	"table":        "<table>",
	"row":          "<tr>",
	"rowgroup":     "<tbody>",
	"rowheader":    `<th scope="row">`,
	"columnheader": `<th scope="col">`,
	"cell":         "<td>",
	"gridcell":     "<td>",
	"grid":         "<table>",
	"list":         "<ul>",
	"listitem":     "<li>",
	"separator":    "<hr>",
	"term":         "<dt>",
	"caption":      "<caption>",
	"time":         "<time>",
	"paragraph":    "<p>",
	"blockquote":   "<blockquote>",
	"generic":      "no role (the implicit role is enough)",
	"link":         `<a href>`,
	"group":        "<fieldset> or <details>",
}

// The implicit role of each tag — if the author's `role` matches,
// the role is redundant rather than a missed semantic.
var implicitRoleOf = map[string]string{
	"a":          "link",
	"article":    "article",
	"aside":      "complementary",
	"button":     "button",
	"footer":     "contentinfo",
	"form":       "form",
	"h1":         "heading",
	"h2":         "heading",
	"h3":         "heading",
	"h4":         "heading",
	"h5":         "heading",
	"h6":         "heading",
	"hr":         "separator",
	"li":         "listitem",
	"main":       "main",
	"nav":        "navigation",
	"ol":         "list",
	"ul":         "list",
	"p":          "paragraph",
	"section":    "region",
	"table":      "table",
	"tbody":      "rowgroup",
	"thead":      "rowgroup",
	"tfoot":      "rowgroup",
	"td":         "cell",
	"th":         "cell",
	"tr":         "row",
	"caption":    "caption",
	"time":       "time",
	"blockquote": "blockquote",
	"dt":         "term",
	"figure":     "figure",
	"search":     "search",
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	r := jsxutil.FindAttribute(attrs, "role")
	if r == nil {
		return
	}
	role, ok := jsxutil.AttributeStringValue(r)
	if !ok {
		return
	}
	suggestion, hasSuggestion := semanticFor[role]
	if !hasSuggestion {
		return
	}
	// If the tag's implicit role already matches, nothing to suggest.
	if implicitRoleOf[tag] == role {
		return
	}
	// Constrained input: the role can be expressed natively but only
	// when paired with a specific `type=`. e.g. <input role="checkbox">
	// is fine iff type="checkbox".
	if tag == "input" {
		t := ""
		if ta := jsxutil.FindAttribute(attrs, "type"); ta != nil {
			t, _ = jsxutil.AttributeStringValue(ta)
		}
		switch role {
		case "checkbox":
			if t == "checkbox" {
				return
			}
		case "radio":
			if t == "radio" {
				return
			}
		case "searchbox":
			if t == "search" {
				return
			}
		case "textbox":
			if t == "text" || t == "" {
				return
			}
		}
	}
	// Constrained <th>: role="rowheader" needs scope="row"; columnheader needs scope="col".
	if tag == "th" {
		scope := ""
		if s := jsxutil.FindAttribute(attrs, "scope"); s != nil {
			scope, _ = jsxutil.AttributeStringValue(s)
		}
		if role == "rowheader" && scope == "row" {
			return
		}
		if role == "columnheader" && scope == "col" {
			return
		}
	}
	ctx.Report(r, "use the semantic element "+suggestion+" instead of role=\""+role+"\"")
}
