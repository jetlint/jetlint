// Command oxlint-fixtures extracts test cases from oxc rule source
// files and writes them as per-rule JSON files for jetlint's
// eslintcompat harness to consume.
//
// Usage:
//
//	oxlint-fixtures --oxc PATH --out DIR --rule RULE [--rule RULE ...]
//
// The --oxc flag points at a checkout of github.com/oxc-project/oxc.
// Each --rule argument is the kebab-case ESLint rule ID; the tool
// maps it to oxc's snake_case file name
// (crates/oxc_linter/src/rules/eslint/<snake>.rs). Output JSON files
// land at <out>/<rule>.json.
//
// This is a developer tool, not a runtime dependency: re-run when
// oxc updates or when jetlint adds a new rule that needs upstream
// fixtures. The committed JSON files under testdata/eslint/ are the
// durable artifact.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "oxlint-fixtures:", err)
		os.Exit(1)
	}
}

// multiFlag implements flag.Value for repeatable --rule flags.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }

func run(args []string) error {
	fs := flag.NewFlagSet("oxlint-fixtures", flag.ContinueOnError)
	oxcPath := fs.String("oxc", "", "path to oxc checkout (required)")
	outDir := fs.String("out", "testdata/eslint", "directory to write JSON fixture files into")
	var rules multiFlag
	fs.Var(&rules, "rule", "ESLint rule ID to extract (repeatable; required at least once)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *oxcPath == "" {
		return fmt.Errorf("--oxc is required")
	}
	if len(rules) == 0 {
		return fmt.Errorf("at least one --rule is required")
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	for _, rule := range rules {
		if err := extractRule(*oxcPath, *outDir, rule); err != nil {
			return fmt.Errorf("rule %s: %w", rule, err)
		}
	}
	return nil
}

// Fixture is the shape jetlint's eslintcompat package loads. Each
// case carries the source code and a Valid flag; pass/fail counts
// fall out of the slice contents.
type Fixture struct {
	RuleID    string `json:"ruleId"`
	OxcSHA    string `json:"oxcSHA,omitempty"`
	OxcSource string `json:"oxcSource"`
	Cases     []Case `json:"cases"`
}

type Case struct {
	Code string `json:"code"`
	// Valid is true for entries from oxc's `pass` vec.
	Valid bool `json:"valid"`
	// Options is the ESLint-style options array for this case
	// (`[options0, options1, ...]`). Nil when the upstream case had no
	// options or used `None`. Rules typically read Options[0].
	Options []any `json:"options,omitempty"`
}

func extractRule(oxcPath, outDir, ruleID string) error {
	snake := strings.ReplaceAll(ruleID, "-", "_")
	// oxc stores most rules as `<rule>.rs`, but rules with multi-file
	// implementations live in `<rule>/mod.rs`. Check both.
	candidates := []string{
		filepath.Join(oxcPath, "crates", "oxc_linter", "src", "rules", "eslint", snake+".rs"),
		filepath.Join(oxcPath, "crates", "oxc_linter", "src", "rules", "eslint", snake, "mod.rs"),
	}
	var (
		src string
		raw []byte
		err error
	)
	for _, c := range candidates {
		raw, err = os.ReadFile(c)
		if err == nil {
			src = c
			break
		}
	}
	if err != nil {
		return fmt.Errorf("read %s (or mod.rs variant): %w", candidates[0], err)
	}
	oxcRelSrc, _ := filepath.Rel(oxcPath, src)
	pass, fail, err := Extract(string(raw))
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	fx := Fixture{
		RuleID:    ruleID,
		OxcSource: oxcRelSrc,
		Cases:     make([]Case, 0, len(pass)+len(fail)),
	}
	for _, c := range pass {
		fx.Cases = append(fx.Cases, Case{Code: c.Code, Valid: true, Options: c.Options})
	}
	for _, c := range fail {
		fx.Cases = append(fx.Cases, Case{Code: c.Code, Valid: false, Options: c.Options})
	}
	// Capture upstream SHA if the checkout is a git repo, so the
	// fixture file records the exact source revision it was generated
	// from.
	if sha, err := readGitSHA(oxcPath); err == nil {
		fx.OxcSHA = sha
	}

	out := filepath.Join(outDir, ruleID+".json")
	body, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(out, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("%s: %d pass, %d fail -> %s\n", ruleID, len(pass), len(fail), out)
	return nil
}

func readGitSHA(dir string) (string, error) {
	head := filepath.Join(dir, ".git", "HEAD")
	data, err := os.ReadFile(head)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(data))
	if rest, ok := strings.CutPrefix(ref, "ref: "); ok {
		refPath := filepath.Join(dir, ".git", rest)
		shaData, err := os.ReadFile(refPath)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(shaData)), nil
	}
	return ref, nil
}
