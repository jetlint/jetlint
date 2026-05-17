// Package nosecrets implements no-secrets: flag string literals that
// look like credentials or API tokens. The detector matches well-known
// secret prefixes (AWS, GitHub, Slack, JWTs, RSA PEM blocks, etc.) —
// the patterns that produce the lowest false-positive rate on real
// codebases. High-entropy heuristics are deliberately avoided because
// they fire on tailwind class strings, css selectors, and similar.
package nosecrets

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-secrets"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindStringLiteral:                 visit,
		wrapperchecker.KindNoSubstitutionTemplateLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	text := n.LiteralText()
	if looksLikeSecret(text) {
		ctx.Report(n, "string literal looks like a secret — store in env or a secrets manager")
	}
}

// looksLikeSecret applies a curated set of high-precision prefix /
// substring checks. The bar is "would a reviewer mark this as a
// leaked credential at a glance?".
func looksLikeSecret(s string) bool {
	if len(s) < 16 {
		return false
	}
	// JWT: three base64-url segments separated by dots, starting
	// with the standard JOSE header `eyJ`.
	if strings.HasPrefix(s, "eyJ") && strings.Count(s, ".") == 2 {
		return true
	}
	// AWS access key id.
	if strings.HasPrefix(s, "AKIA") && len(s) >= 20 {
		return true
	}
	// Slack tokens.
	if strings.HasPrefix(s, "xoxb-") || strings.HasPrefix(s, "xoxa-") ||
		strings.HasPrefix(s, "xoxp-") || strings.HasPrefix(s, "xoxs-") ||
		strings.HasPrefix(s, "xoxo-") {
		return true
	}
	// Slack webhook URL.
	if strings.Contains(s, "hooks.slack.com/services/") {
		return true
	}
	// GitHub personal access tokens (classic and fine-grained).
	if strings.HasPrefix(s, "ghp_") || strings.HasPrefix(s, "github_pat_") ||
		strings.HasPrefix(s, "gho_") || strings.HasPrefix(s, "ghu_") ||
		strings.HasPrefix(s, "ghs_") || strings.HasPrefix(s, "ghr_") {
		return true
	}
	// Stripe / OpenAI live keys.
	if strings.HasPrefix(s, "sk_live_") || strings.HasPrefix(s, "rk_live_") {
		return true
	}
	if strings.HasPrefix(s, "sk-") && len(s) >= 32 {
		return true
	}
	// Twilio account / api keys begin with AC / SK followed by 32 hex.
	if (strings.HasPrefix(s, "SK") || strings.HasPrefix(s, "AC")) && len(s) >= 32 && isAlnum(s[2:]) {
		return true
	}
	// PEM private key blocks.
	if strings.Contains(s, "-----BEGIN") && strings.Contains(s, "PRIVATE KEY") {
		return true
	}
	// Token-naming markers explicit in the literal itself.
	if strings.HasPrefix(s, "facebook_app_id_") ||
		strings.HasPrefix(s, "twitter_api_key_") {
		return true
	}
	return false
}

func isAlnum(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}
