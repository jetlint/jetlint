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
	// Rules maps a rule identifier to a severity ("error", "warning",
	// "off"). Absence means "no opinion at this level"; cascade resolution
	// then defers to the parent.
	Rules map[string]string `json:"rules,omitempty"`
}

// ResolvedConfig is the effective configuration for a particular file
// after cascade resolution. The Rules map names exactly the rules that
// will produce diagnostics, with their effective severity. Rules set to
// "off" at any level are absent from the map.
type ResolvedConfig struct {
	Rules map[string]wrapperlint.Severity
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
	for ruleID, sev := range cfg.Rules {
		if !rules.IsKnown(ruleID) {
			return FileConfig{}, toolerr.WithPath(toolerr.CodeConfigUnknownRule,
				fmt.Sprintf("unknown rule %q in %s", ruleID, path), path)
		}
		if !validSeverity(sev) {
			return FileConfig{}, toolerr.WithPath(toolerr.CodeConfigInvalid,
				fmt.Sprintf("rule %q has invalid severity %q (expected error, warning, or off)", ruleID, sev), path)
		}
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
		Rules: defaultRules(),
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
// severity for that rule, including the explicit "off" value which
// removes the rule from the resolved set.
func mergeFileInto(resolved *ResolvedConfig, child FileConfig) {
	for ruleID, sev := range child.Rules {
		if sev == "off" {
			delete(resolved.Rules, ruleID)
			continue
		}
		resolved.Rules[ruleID] = wrapperlint.Severity(sev)
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

