// Command biome-fixtures extracts test cases from biome's per-rule test
// spec directories and writes them as per-rule JSON files for jetlint's
// eslintcompat harness to consume.
//
// Usage:
//
//	biome-fixtures --biome PATH --out DIR --rule RULE --category CAT [--rule RULE --category CAT ...]
//
// The --biome flag points at a checkout of github.com/biomejs/biome.
// Each --rule argument is the kebab-case ESLint-style rule ID; the tool
// maps it to biome's camelCase directory layout (e.g. no-precision-loss
// -> noPrecisionLoss). The --category flag names biome's category
// subdirectory (correctness, suspicious, performance, ...). Output JSON
// files land at <out>/<rule>.json.
//
// Biome's test format: each rule directory holds pairs of source files
// and snapshot files, e.g. valid.js / valid.js.snap, invalid.ts /
// invalid.ts.snap, valid_01.tsx / valid_01.tsx.snap. The filename
// stem encodes validity — files containing "invalid" produce
// diagnostics; everything else passes clean. Snapshot files are
// metadata-only here; we read them only to confirm the file is a
// genuine test fixture (vs a stray helper).
//
// This is a developer tool, not a runtime dependency: re-run when
// biome updates a rule's tests or when jetlint adds a new biome-derived
// rule. The committed JSON files under testdata/biome/ are the durable
// artifact.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "biome-fixtures:", err)
		os.Exit(1)
	}
}

// pair holds parallel slices of --rule and --category flags so the
// Nth rule pairs with the Nth category. flag.Value lets either be
// repeated; we error out if the lengths diverge.
type pair struct {
	values *[]string
}

func (p pair) String() string { return strings.Join(*p.values, ",") }
func (p pair) Set(s string) error {
	*p.values = append(*p.values, s)
	return nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("biome-fixtures", flag.ContinueOnError)
	biomePath := fs.String("biome", "", "path to biome checkout (required)")
	outDir := fs.String("out", "testdata/biome", "directory to write JSON fixture files into")
	var rules []string
	var categories []string
	fs.Var(pair{&rules}, "rule", "ESLint-style kebab-case rule ID (repeatable, paired with --category)")
	fs.Var(pair{&categories}, "category", "biome category subdirectory (repeatable, paired with --rule)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *biomePath == "" {
		return fmt.Errorf("--biome is required")
	}
	if len(rules) == 0 {
		return fmt.Errorf("at least one --rule is required")
	}
	if len(rules) != len(categories) {
		return fmt.Errorf("--rule and --category must be paired (got %d rules, %d categories)",
			len(rules), len(categories))
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	for i, rule := range rules {
		if err := extractRule(*biomePath, *outDir, rule, categories[i]); err != nil {
			return fmt.Errorf("rule %s: %w", rule, err)
		}
	}
	return nil
}

// Fixture is the shape jetlint's eslintcompat package loads. Identical
// to the oxlint-fixtures schema so the same loader handles both
// sources; the BiomeSource field replaces OxcSource for traceability.
type Fixture struct {
	RuleID      string `json:"ruleId"`
	BiomeSHA    string `json:"biomeSHA,omitempty"`
	BiomeSource string `json:"biomeSource"`
	Cases       []Case `json:"cases"`
}

// Case mirrors eslintcompat.Case. Options and Settings are present in
// the schema for compatibility but unused: biome encodes per-test rule
// options inside its own configuration files we don't parse.
type Case struct {
	Code     string         `json:"code"`
	Valid    bool           `json:"valid"`
	Options  []any          `json:"options,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
}

// toCamelCase converts a kebab-case rule ID to the camelCase form biome
// uses for directory names. Example: "no-precision-loss" -> "noPrecisionLoss".
func toCamelCase(kebab string) string {
	parts := strings.Split(kebab, "-")
	if len(parts) == 0 {
		return kebab
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

// classifyValidity reports whether biome's snapshot file for this
// fixture records any diagnostics. A `# Diagnostics` section in the
// `.snap` file means the rule fires (so the test is invalid); its
// absence means the file passes clean. This is the source of truth
// — biome's filename conventions don't cover regression fixtures
// like `issue-NNNN.tsx`.
func classifyValidity(snapBody []byte) bool {
	if snapBody == nil {
		return true
	}
	return !bytes.Contains(snapBody, []byte("\n# Diagnostics\n"))
}


// extractRule walks one rule's spec directory under biome's test tree
// and emits a JSON fixture file. The directory layout is fixed by
// biome's snapshot-test convention; we don't need biome's category
// taxonomy except to find the right directory.
func extractRule(biomePath, outDir, ruleID, category string) error {
	camel := toCamelCase(ruleID)
	ruleDir := filepath.Join(biomePath, "crates", "biome_js_analyze", "tests", "specs", category, camel)
	entries, err := os.ReadDir(ruleDir)
	if err != nil {
		return fmt.Errorf("read spec dir %s: %w", ruleDir, err)
	}

	// Pair each non-snap file with its .snap. Files without a snap
	// neighbor are helpers (e.g. shared imports) and are skipped.
	hasSnap := make(map[string]bool, len(entries))
	for _, e := range entries {
		if stem, ok := strings.CutSuffix(e.Name(), ".snap"); ok {
			hasSnap[stem] = true
		}
	}
	// Files with a sibling `<stem>.*.json` companion carry per-test
	// configuration this extractor doesn't parse — rule options
	// (`.options.json`), an emulated package.json
	// (`.package.json`), and similar. Without applying the
	// companion the fixture would run under default options and
	// usually produce the wrong diagnostic count, so skip the
	// whole file.
	hasOptions := make(map[string]bool)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonc") {
			continue
		}
		// Find the `<stem>` part by stripping the LAST two
		// extensions: `.<flavor>.json` → `<stem>`. If only one
		// extension exists this is a JSON fixture (we read it
		// as a code source), not a companion.
		dotJSON := strings.TrimSuffix(name, ".json")
		dot := strings.LastIndexByte(dotJSON, '.')
		if dot < 0 {
			continue
		}
		stem := dotJSON[:dot]
		for _, f := range entries {
			fn := f.Name()
			if strings.HasSuffix(fn, ".snap") {
				continue
			}
			fnStem := strings.TrimSuffix(fn, filepath.Ext(fn))
			if fnStem == stem {
				hasOptions[fn] = true
			}
		}
	}

	biomeRelSrc, _ := filepath.Rel(biomePath, ruleDir)
	fx := Fixture{
		RuleID:      ruleID,
		BiomeSource: biomeRelSrc,
		Cases:       make([]Case, 0, len(hasSnap)),
	}

	// Deduplicate identical sources. Biome occasionally lists the same
	// source under multiple filenames when it's testing config variations
	// rather than source variations; first occurrence wins.
	seen := make(map[string]struct{}, len(hasSnap))
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".snap") {
			continue
		}
		if !hasSnap[name] {
			continue
		}
		if hasOptions[name] {
			continue
		}
		body, err := os.ReadFile(filepath.Join(ruleDir, name))
		if err != nil {
			return fmt.Errorf("read fixture %s: %w", name, err)
		}
		snapBody, _ := os.ReadFile(filepath.Join(ruleDir, name+".snap"))
		valid := classifyValidity(snapBody)
		// JSONC files contain a top-level array of code strings — biome
		// runs each string as its own test case. The raw JSONC text isn't
		// valid JS, so leaving it whole would produce a parse error
		// instead of a real per-snippet check. Decode here and emit one
		// case per element.
		if strings.HasSuffix(name, ".jsonc") || strings.HasSuffix(name, ".json") {
			snippets, ok := decodeJSONCStringArray(body)
			if !ok {
				return fmt.Errorf("fixture %s is JSONC but not a string array", name)
			}
			for _, code := range snippets {
				if _, dup := seen[code]; dup {
					continue
				}
				seen[code] = struct{}{}
				fx.Cases = append(fx.Cases, Case{Code: code, Valid: valid})
			}
			continue
		}
		code := string(body)
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		fx.Cases = append(fx.Cases, Case{Code: code, Valid: valid})
	}

	if sha, err := readGitSHA(biomePath); err == nil {
		fx.BiomeSHA = sha
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
	fmt.Printf("%s: %d cases -> %s\n", ruleID, len(fx.Cases), out)
	return nil
}

// decodeJSONCStringArray parses a JSONC byte slice as a top-level
// array of strings, stripping `//` and `/* */` comments first so
// real-world biome fixtures (which often include `// Notes`) decode
// cleanly. Returns ok=false if the body isn't a string array.
func decodeJSONCStringArray(body []byte) ([]string, bool) {
	cleaned := stripJSONCComments(body)
	var snippets []string
	if err := json.Unmarshal(cleaned, &snippets); err != nil {
		return nil, false
	}
	return snippets, true
}

// stripJSONCComments removes // line comments and /* block comments,
// taking care not to misinterpret comment-like sequences inside JSON
// string literals. Sufficient for biome's fixture JSONC files; not a
// general-purpose JSONC parser.
func stripJSONCComments(body []byte) []byte {
	out := make([]byte, 0, len(body))
	inString := false
	escape := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if inString {
			out = append(out, c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(body) {
			if body[i+1] == '/' {
				for i < len(body) && body[i] != '\n' {
					i++
				}
				if i < len(body) {
					out = append(out, body[i])
				}
				continue
			}
			if body[i+1] == '*' {
				i += 2
				for i+1 < len(body) && !(body[i] == '*' && body[i+1] == '/') {
					i++
				}
				i++ // land on '/'
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// readGitSHA reads the .git/HEAD ref the same way oxlint-fixtures does
// so the fixture file records the exact biome revision it was generated
// from. Returns an empty string when the path isn't a git checkout.
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
