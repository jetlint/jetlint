// Package usevalidautocomplete implements use-valid-autocomplete:
// the `autocomplete` attribute on form controls must use values
// defined in the HTML spec (e.g. "email", "current-password"). A
// typo silently disables browser autofill.
package usevalidautocomplete

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-valid-autocomplete"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// HTML spec autocomplete tokens.
var validTokens = map[string]bool{
	"on": true, "off": true,
	"name": true, "honorific-prefix": true, "given-name": true,
	"additional-name": true, "family-name": true, "honorific-suffix": true,
	"nickname": true, "email": true, "username": true, "new-password": true,
	"current-password": true, "one-time-code": true, "organization-title": true,
	"organization": true, "street-address": true, "address-line1": true,
	"address-line2": true, "address-line3": true, "address-level4": true,
	"address-level3": true, "address-level2": true, "address-level1": true,
	"country": true, "country-name": true, "postal-code": true,
	"cc-name": true, "cc-given-name": true, "cc-additional-name": true,
	"cc-family-name": true, "cc-number": true, "cc-exp": true,
	"cc-exp-month": true, "cc-exp-year": true, "cc-csc": true, "cc-type": true,
	"transaction-currency": true, "transaction-amount": true, "language": true,
	"bday": true, "bday-day": true, "bday-month": true, "bday-year": true,
	"sex": true, "tel": true, "tel-country-code": true, "tel-national": true,
	"tel-area-code": true, "tel-local": true, "tel-extension": true,
	"impp": true, "url": true, "photo": true, "webauthn": true,
	"shipping": true, "billing": true,
	"home": true, "work": true, "mobile": true, "fax": true, "pager": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if tag != "input" && tag != "textarea" && tag != "select" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	attr := jsxutil.FindAttribute(attrs, "autoComplete")
	if attr == nil {
		attr = jsxutil.FindAttribute(attrs, "autocomplete")
	}
	if attr == nil {
		return
	}
	v, ok := jsxutil.AttributeStringValue(attr)
	if !ok {
		return
	}
	for t := range strings.FieldsSeq(v) {
		if !validTokens[strings.ToLower(t)] {
			ctx.Report(attr, "autocomplete=\""+v+"\" includes the invalid token \""+t+"\"")
			return
		}
	}
}
