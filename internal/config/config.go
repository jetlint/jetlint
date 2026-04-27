// Package config loads and resolves linter configuration. The on-disk
// format is JSON for v0.1; a future revision can add JSONC if user
// feedback warrants the parser dependency. Configurations cascade up
// the directory tree from the target file: closer files override
// farther ones with replace semantics for list-typed settings.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/tommymorgan/tsgolint/internal/rules"
	"github.com/tommymorgan/tsgolint/internal/toolerr"
)

// ConfigFileName is the filename the resolver looks for at every level
// of the cascade. The leading dot keeps it out of casual `ls` output and
// matches the convention of similar tools.
const ConfigFileName = ".tsgolintrc.json"

// FileConfig is the on-disk representation of a single configuration
// file. Fields are pointers so the resolver can distinguish "absent"
// from "explicitly empty" during cascade merging.
type FileConfig struct {
	// Rules maps a rule identifier to either a bare severity string
	// (`"error"`, `"warning"`, `"off"`) or a tuple
	// `["<severity>", { ...options }]` for rules that accept options.
	// Absence means "no opinion at this level"; cascade resolution
	// then defers to the parent.
	Rules map[string]RuleEntry `json:"rules,omitempty"`

	// MaxDiagnostics caps the number of diagnostic blocks the human
	// formatter renders. A pointer distinguishes "absent" (defer to
	// parent / default) from "0" (explicit "render everything").
	MaxDiagnostics *int `json:"maxDiagnostics,omitempty"`
}

// RuleEntry is one rule's configuration: a severity plus optional
// rule-specific options. The on-disk JSON accepts two shapes —
// `"error"` (severity only) or `["error", { ...options }]` (severity
// plus options) — matching typescript-eslint's convention.
type RuleEntry struct {
	Severity string
	// Options is the raw JSON of the options object, or nil when no
	// options were provided. Each rule parses its own raw options at
	// rule-construction time so the config layer doesn't need to know
	// the option schema of every shipped rule.
	Options json.RawMessage
}

// UnmarshalJSON accepts either `"<severity>"` or
// `["<severity>", { ...options }]`. Any other shape is a parse error.
func (r *RuleEntry) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Severity = s
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return errors.New("rule entry must be a severity string or [severity, options] array")
	}
	if len(arr) == 0 {
		return errors.New("rule entry array must contain at least a severity")
	}
	if err := json.Unmarshal(arr[0], &r.Severity); err != nil {
		return errors.New("rule entry first element must be a severity string")
	}
	if len(arr) >= 2 {
		r.Options = arr[1]
	}
	return nil
}

// DefaultMaxDiagnostics is the truncation threshold applied when no
// configuration file specifies a value. Matches biome's default so
// users get the same "show 20, hide the rest" experience across tools.
const DefaultMaxDiagnostics = 20

// ResolvedConfig is the effective configuration for a particular file
// after cascade resolution. The Rules map names exactly the rules that
// will produce diagnostics, with their effective severity. Rules set to
// "off" at any level are absent from the map.
type ResolvedConfig struct {
	Rules map[string]wrapperlint.Severity
	// RuleOptions is the per-rule raw JSON options for rules that have
	// any. Rules without options are absent from the map; rule
	// constructors must treat absent and empty raw-message as
	// equivalent. Cascade semantics: child wins (replace, not merge).
	RuleOptions    map[string]json.RawMessage
	MaxDiagnostics int
	// Sources lists the configuration files that contributed to this
	// resolution, in cascade order (outermost first). Useful for "why is
	// this rule active?" debugging in future tooling.
	Sources []string
}

// LoadFile reads and validates a single configuration file. It returns
// a structured tooling error on parse or validation failure so the CLI
// can route it through the JSON-mode error contract.
func LoadFile(path string) (FileConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, toolerr.WithPath(toolerr.CodeConfigInvalid, err.Error(), path)
	}
	var cfg FileConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return FileConfig{}, toolerr.WithPath(toolerr.CodeConfigInvalid,
			fmt.Sprintf("parse %s: %v", path, err), path)
	}
	for ruleID, entry := range cfg.Rules {
		if !rules.IsKnown(ruleID) {
			return FileConfig{}, toolerr.WithPath(toolerr.CodeConfigUnknownRule,
				fmt.Sprintf("unknown rule %q in %s", ruleID, path), path)
		}
		if !validSeverity(entry.Severity) {
			return FileConfig{}, toolerr.WithPath(toolerr.CodeConfigInvalid,
				fmt.Sprintf("rule %q has invalid severity %q (expected error, warning, or off)", ruleID, entry.Severity), path)
		}
	}
	if cfg.MaxDiagnostics != nil && *cfg.MaxDiagnostics < 0 {
		return FileConfig{}, toolerr.WithPath(toolerr.CodeConfigInvalid,
			fmt.Sprintf("maxDiagnostics in %s must be non-negative (use 0 to disable truncation)", path), path)
	}
	return cfg, nil
}

// ResolveCascade walks upward from startDir, collecting every
// .tsgolintrc.json it finds, and merges them with child-wins replace
// semantics. The walk stops at the filesystem root. Built-in defaults
// supply the floor: every shipped rule is active at error severity
// unless overridden.
func ResolveCascade(startDir string) (ResolvedConfig, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return ResolvedConfig{}, toolerr.New(toolerr.CodeInternal, err.Error())
	}

	// Collect candidate configuration files from the filesystem root
	// downward, so the cascade merge proceeds outermost-to-innermost.
	var stack []string
	dir := abs
	for {
		candidate := filepath.Join(dir, ConfigFileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			stack = append([]string{candidate}, stack...)
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return ResolvedConfig{}, toolerr.WithPath(toolerr.CodeConfigInvalid, err.Error(), candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	resolved := ResolvedConfig{
		Rules:          defaultRules(),
		RuleOptions:    map[string]json.RawMessage{},
		MaxDiagnostics: DefaultMaxDiagnostics,
	}
	for _, path := range stack {
		cfg, err := LoadFile(path)
		if err != nil {
			return ResolvedConfig{}, err
		}
		mergeFileInto(&resolved, cfg)
		resolved.Sources = append(resolved.Sources, path)
	}
	return resolved, nil
}

func defaultRules() map[string]wrapperlint.Severity {
	out := make(map[string]wrapperlint.Severity, len(rules.MVPRuleIDs))
	for _, id := range rules.MVPRuleIDs {
		out[id] = rules.DefaultSeverity(id)
	}
	return out
}

// mergeFileInto applies a single FileConfig as a cascade override.
// Replace semantics: any rule mentioned in the child sets the effective
// severity (and options) for that rule, including the explicit "off"
// value which removes the rule from the resolved set. Options are
// replaced wholesale on each child entry — the rule that mentions
// options at the deepest level wins; mixing parent-options with
// child-severity is not supported (the user can replicate the parent's
// options in the child if they want both).
func mergeFileInto(resolved *ResolvedConfig, child FileConfig) {
	for ruleID, entry := range child.Rules {
		if entry.Severity == "off" {
			delete(resolved.Rules, ruleID)
			delete(resolved.RuleOptions, ruleID)
			continue
		}
		resolved.Rules[ruleID] = wrapperlint.Severity(entry.Severity)
		if len(entry.Options) > 0 {
			resolved.RuleOptions[ruleID] = entry.Options
		} else {
			delete(resolved.RuleOptions, ruleID)
		}
	}
	if child.MaxDiagnostics != nil {
		resolved.MaxDiagnostics = *child.MaxDiagnostics
	}
}

func validSeverity(s string) bool {
	switch s {
	case "error", "warning", "off":
		return true
	default:
		return false
	}
}

