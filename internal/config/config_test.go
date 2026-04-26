package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/config"
	"github.com/tommymorgan/tsgolint/internal/toolerr"
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

func TestResolveCascade_NoConfigUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := config.ResolveCascade(dir)
	if err != nil {
		t.Fatalf("ResolveCascade: %v", err)
	}
	for _, ruleID := range []string{
		"no-floating-promises",
		"no-misused-promises",
		"strict-boolean-expressions",
		"no-unsafe-assignment",
		"no-base-to-string",
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
	writeFile(t, filepath.Join(dir, ".tsgolintrc.json"),
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
	writeFile(t, filepath.Join(root, ".tsgolintrc.json"),
		`{"rules": {"no-floating-promises": "warning"}}`)
	child := filepath.Join(root, "packages", "web")
	writeFile(t, filepath.Join(child, ".tsgolintrc.json"),
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
	writeFile(t, filepath.Join(dir, ".tsgolintrc.json"), `{not valid json`)
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
	writeFile(t, filepath.Join(dir, ".tsgolintrc.json"),
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
	writeFile(t, filepath.Join(dir, ".tsgolintrc.json"),
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
	writeFile(t, filepath.Join(root, ".tsgolintrc.json"),
		`{"rules": {"no-floating-promises": "warning", "no-misused-promises": "off"}}`)
	child := filepath.Join(root, "child")
	writeFile(t, filepath.Join(child, ".tsgolintrc.json"),
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
