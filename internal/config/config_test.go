package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/config"
	"github.com/jetlint/jetlint/internal/toolerr"
)

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestResolveCascade_DefaultMaxDiagnosticsMatchesBiome(t *testing.T) {
	dir := t.TempDir()
	got, err := config.ResolveCascade(dir)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	// Biome's default is 20; we match that value so users hitting
	// either tool see the same "shows 20, hides the rest" behavior.
	if got.MaxDiagnostics != 20 {
		t.Errorf("expected default MaxDiagnostics=20 (matching biome), got %d", got.MaxDiagnostics)
	}
}

func TestResolveCascade_RootConfigOverridesMaxDiagnostics(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"),
		`{"maxDiagnostics": 50}`)
	got, err := config.ResolveCascade(dir)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	if got.MaxDiagnostics != 50 {
		t.Errorf("expected MaxDiagnostics=50 from config, got %d", got.MaxDiagnostics)
	}
}

func TestResolveCascade_ChildConfigOverridesParentMaxDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".jetlintrc.json"),
		`{"maxDiagnostics": 50}`)
	child := filepath.Join(root, "packages", "web")
	writeFile(t, filepath.Join(child, ".jetlintrc.json"),
		`{"maxDiagnostics": 5}`)
	got, err := config.ResolveCascade(child)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	if got.MaxDiagnostics != 5 {
		t.Errorf("expected child MaxDiagnostics=5 to win, got %d", got.MaxDiagnostics)
	}
}

func TestResolveCascade_MaxDiagnosticsZeroDisablesTruncation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"),
		`{"maxDiagnostics": 0}`)
	got, err := config.ResolveCascade(dir)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	// Explicit 0 must be preserved (sentinel for "unlimited"), not
	// silently treated as "unset" and replaced with the default.
	if got.MaxDiagnostics != 0 {
		t.Errorf("expected explicit MaxDiagnostics=0 to disable truncation, got %d", got.MaxDiagnostics)
	}
}

func TestResolveCascade_NegativeMaxDiagnosticsRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"),
		`{"maxDiagnostics": -1}`)
	_, err := config.ResolveCascade(dir)
	if err == nil {
		t.Fatal("expected error for negative maxDiagnostics, got nil")
	}
}

func TestResolveCascade_NoConfigUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := config.ResolveCascade(dir)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	for _, ruleID := range []string{
		"await-thenable",
		"no-base-to-string",
		"no-floating-promises",
		"no-misused-promises",
		"no-unsafe-assignment",
	} {
		if got.Rules[ruleID] != wrapperlint.SeverityError {
			t.Errorf("expected %s at error severity by default, got %q", ruleID, got.Rules[ruleID])
		}
	}
	if len(got.Sources) != 0 {
		t.Errorf("expected no source files, got %v", got.Sources)
	}
}

func TestResolveCascade_RootConfigOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": "warning"}}`)
	got, err := config.ResolveCascade(dir)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	if got.Rules["no-floating-promises"] != wrapperlint.SeverityWarning {
		t.Errorf("expected warning, got %q", got.Rules["no-floating-promises"])
	}
	// Other defaults remain.
	if got.Rules["no-misused-promises"] != wrapperlint.SeverityError {
		t.Errorf("expected unchanged error, got %q", got.Rules["no-misused-promises"])
	}
}

func TestResolveCascade_ChildConfigOverridesParent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": "warning"}}`)
	child := filepath.Join(root, "packages", "web")
	writeFile(t, filepath.Join(child, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": "off"}}`)

	got, err := config.ResolveCascade(child)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	if _, present := got.Rules["no-floating-promises"]; present {
		t.Errorf("expected rule disabled in child subtree, got severity %q",
			got.Rules["no-floating-promises"])
	}
	if len(got.Sources) != 2 {
		t.Errorf("expected two sources in cascade, got %d: %v", len(got.Sources), got.Sources)
	}
}

func TestResolveCascade_InvalidJSONReturnsConfigInvalidError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"), `{not valid json`)
	_, err := config.ResolveCascade(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var te *toolerr.Error
	if !errors.As(err, &te) {
		t.Fatalf("expected *toolerr.Error, got %T: %v", err, err)
	}
	if te.Code != toolerr.CodeConfigInvalid {
		t.Errorf("expected code %s, got %s", toolerr.CodeConfigInvalid, te.Code)
	}
	if te.Path == "" {
		t.Errorf("expected path on error, got empty")
	}
}

func TestResolveCascade_UnknownRuleReturnsConfigUnknownRuleError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"),
		`{"rules": {"made-up-rule": "error"}}`)
	_, err := config.ResolveCascade(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var te *toolerr.Error
	if !errors.As(err, &te) {
		t.Fatalf("expected *toolerr.Error, got %T: %v", err, err)
	}
	if te.Code != toolerr.CodeConfigUnknownRule {
		t.Errorf("expected code %s, got %s", toolerr.CodeConfigUnknownRule, te.Code)
	}
}

func TestResolveCascade_InvalidSeverityReturnsConfigInvalidError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": "fatal"}}`)
	_, err := config.ResolveCascade(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var te *toolerr.Error
	if !errors.As(err, &te) {
		t.Fatalf("expected *toolerr.Error, got %T: %v", err, err)
	}
	if te.Code != toolerr.CodeConfigInvalid {
		t.Errorf("expected code %s, got %s", toolerr.CodeConfigInvalid, te.Code)
	}
}

func TestResolveCascade_ChildLevelListReplacesParentLevelList(t *testing.T) {
	// In v0.1, lists (the rules map is the only "list" we have at present)
	// follow replace semantics: child rule entries entirely supersede
	// parent entries for that rule, with no concatenation. Disabling a
	// rule in a child cleanly removes it; re-enabling at child re-asserts.
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": "warning", "no-misused-promises": "off"}}`)
	child := filepath.Join(root, "child")
	writeFile(t, filepath.Join(child, ".jetlintrc.json"),
		`{"rules": {"no-misused-promises": "warning"}}`)

	got, err := config.ResolveCascade(child)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	// Parent's no-floating-promises override (warning) survives because child
	// did not mention it.
	if got.Rules["no-floating-promises"] != wrapperlint.SeverityWarning {
		t.Errorf("parent override should survive in absence of child opinion; got %q",
			got.Rules["no-floating-promises"])
	}
	// Parent disabled no-misused-promises; child re-enables at warning.
	if got.Rules["no-misused-promises"] != wrapperlint.SeverityWarning {
		t.Errorf("child re-enable should override parent disable; got %q",
			got.Rules["no-misused-promises"])
	}
}

func TestResolveCascade_RuleEntryArrayCarriesOptions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": ["error", {"ignoreVoid": false}]}}`)
	got, err := config.ResolveCascade(dir)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	if got.Rules["no-floating-promises"] != wrapperlint.SeverityError {
		t.Errorf("expected severity 'error', got %q", got.Rules["no-floating-promises"])
	}
	rawOpts := got.RuleOptions["no-floating-promises"]
	if len(rawOpts) == 0 {
		t.Fatal("expected RuleOptions to carry the parsed options for no-floating-promises")
	}
	if string(rawOpts) != `{"ignoreVoid": false}` {
		t.Errorf("unexpected raw options: %s", rawOpts)
	}
}

func TestResolveCascade_ChildArrayShapeReplacesParentScalar(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": "warning"}}`)
	child := filepath.Join(root, "child")
	writeFile(t, filepath.Join(child, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": ["error", {"checkThenables": true}]}}`)
	got, err := config.ResolveCascade(child)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	if got.Rules["no-floating-promises"] != wrapperlint.SeverityError {
		t.Errorf("child severity should win; got %q", got.Rules["no-floating-promises"])
	}
	if string(got.RuleOptions["no-floating-promises"]) != `{"checkThenables": true}` {
		t.Errorf("child options should win; got %s", got.RuleOptions["no-floating-promises"])
	}
}

func TestResolveCascade_OffClearsParentOptions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": ["error", {"ignoreVoid": false}]}}`)
	child := filepath.Join(root, "child")
	writeFile(t, filepath.Join(child, ".jetlintrc.json"),
		`{"rules": {"no-floating-promises": "off"}}`)
	got, err := config.ResolveCascade(child)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	if _, present := got.Rules["no-floating-promises"]; present {
		t.Error("rule should be off")
	}
	if _, present := got.RuleOptions["no-floating-promises"]; present {
		t.Error("options should be cleared when rule is off'd")
	}
}

func TestLoadFile_RejectsRuleEntryWithBadShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".jetlintrc.json")
	writeFile(t, path,
		`{"rules": {"no-floating-promises": 42}}`)
	_, err := config.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for numeric rule entry")
	}
	var te *toolerr.Error
	if !errors.As(err, &te) || te.Code != toolerr.CodeConfigInvalid {
		t.Errorf("expected CodeConfigInvalid tool error, got %v", err)
	}
}
