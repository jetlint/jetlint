package noimportcycles_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/noimportcycles"
)

// tsconfig opens up bundler resolution and allowJs so the cross-file
// import graph the rule traverses includes both `.js` and `.ts`
// fixtures inside the same program.
const tsconfigBody = `{
  "compilerOptions": {
    "strict": false, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true, "allowJs": true, "noImplicitAny": false,
    "allowImportingTsExtensions": true
  },
  "include": ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx"]
}`

// expectation captures what the rule should report against one file
// in the fixture program, plus any non-default options that apply
// only to that file (matching biome's `<name>.options.json` mechanic).
type expectation struct {
	relPath        string
	expectedCount  int
	overrideIgnore *bool // nil = use default rule options
}

// TestNoImportCycles_BiomeCompatibility loads the entire fixture
// directory as a single TypeScript program and lints each file with
// the options biome's spec_tests harness would apply, then verifies
// the rule produces the same diagnostic count per file as biome's
// snapshots.
//
// Per-file options are required because biome runs each test file as
// its own analysis, applying any neighbouring `<stem>.options.json`.
// We mirror that by re-running the rule per file with the right
// Options value rather than running the engine once over the whole
// program (the engine runs all rules with one shared option blob).
func TestNoImportCycles_BiomeCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("compatibility harness skipped under -short")
	}
	fixtureRoot, err := filepath.Abs("../../../testdata/biome/no-import-cycles")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	dir := mirrorFixture(t, fixtureRoot)

	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(tsconfigBody), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}

	includeTypesOpts := loadBoolOption(t,
		filepath.Join(dir, "includeTypes.options.json"),
		"suspicious", "noImportCycles", "ignoreTypes")

	cases := []expectation{
		{"invalidFoobar.js", 1, nil},
		{"invalidBaz.js", 1, nil},
		{"valid.js", 0, nil},
		{"types.ts", 0, nil},
		{"includeTypes.ts", 1, &includeTypesOpts},
		{"ignoreTypes/a.ts", 0, nil},
		{"ignoreTypes/b.ts", 0, nil},
		{"ignoreTypes/c.ts", 0, nil},
	}

	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	defer prog.Close()

	loaded := map[string]struct{}{}
	for _, sf := range prog.SourceFiles() {
		loaded[sf.Path()] = struct{}{}
	}

	var passed, failed int
	for _, c := range cases {
		abs := filepath.Join(dir, c.relPath)
		if _, ok := loaded[abs]; !ok {
			failed++
			t.Errorf("FAIL %s: file not present in loaded program", c.relPath)
			continue
		}
		opts := noimportcycles.DefaultOptions()
		if c.overrideIgnore != nil {
			opts.IgnoreTypes = *c.overrideIgnore
		}
		count := countDiagnosticsForFile(prog, abs, opts)
		if count != c.expectedCount {
			failed++
			t.Errorf("MISMATCH %s: expected %d, got %d",
				c.relPath, c.expectedCount, count)
			continue
		}
		passed++
	}
	total := passed + failed
	pct := 0.0
	if total > 0 {
		pct = float64(passed) * 100.0 / float64(total)
	}
	t.Logf("biome compatibility: %d/%d passed (%.1f%%)", passed, total, pct)
	if failed > 0 {
		t.Fatalf("expected 100%% pass rate, got %d/%d (%.1f%%)", passed, total, pct)
	}
}

// countDiagnosticsForFile builds a fresh engine for one rule instance
// (so the supplied per-file options apply) and returns the number of
// no-import-cycles diagnostics emitted against the given file path.
func countDiagnosticsForFile(prog *wrapperchecker.Program, absPath string,
	opts noimportcycles.Options) int {
	eng := engine.New(
		[]engine.Rule{noimportcycles.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"no-import-cycles": wrapperlint.SeverityError},
	)
	count := 0
	for _, d := range eng.Lint(prog) {
		if d.RuleID != "no-import-cycles" {
			continue
		}
		if d.Range.File != absPath {
			continue
		}
		count++
	}
	return count
}

// mirrorFixture copies the read-only fixture tree into a temp dir so
// the wrapper, which expects a writable project root, has somewhere
// to drop its tsconfig.json alongside the sources.
func mirrorFixture(t *testing.T, src string) string {
	t.Helper()
	dst, err := os.MkdirTemp("/tmp", "jl-noimportcycles")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dst) })
	if err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// Skip the README — it isn't part of the program.
		if filepath.Base(rel) == "README.md" {
			return nil
		}
		return copyFile(path, target)
	}); err != nil {
		t.Fatalf("mirror: %v", err)
	}
	// Sanity check that the well-known fixture files are present.
	want := []string{
		"invalidFoobar.js", "invalidBaz.js", "valid.js",
		"types.ts", "includeTypes.ts",
		"ignoreTypes/a.ts", "ignoreTypes/b.ts", "ignoreTypes/c.ts",
	}
	sort.Strings(want)
	missing := []string{}
	for _, w := range want {
		if _, err := os.Stat(filepath.Join(dst, w)); err != nil {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("fixture missing files: %v", missing)
	}
	return dst
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// loadBoolOption reads biome's nested rule options JSON and returns
// the addressed boolean value. Used to honour `includeTypes.options.
// json`, which sets `ignoreTypes: false` for the includeTypes.ts
// case so its type-only cycle becomes visible.
func loadBoolOption(t *testing.T, path string, category, ruleCamel, key string) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Linter struct {
			Rules map[string]map[string]struct {
				Options map[string]bool `json:"options"`
			} `json:"rules"`
		} `json:"linter"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	cat, ok := doc.Linter.Rules[category]
	if !ok {
		t.Fatalf("options file %s missing category %q", path, category)
	}
	r, ok := cat[ruleCamel]
	if !ok {
		t.Fatalf("options file %s missing rule %q in category %q",
			path, ruleCamel, category)
	}
	val, ok := r.Options[key]
	if !ok {
		t.Fatalf("options file %s missing key %q under %s/%s",
			path, key, category, ruleCamel)
	}
	return val
}
