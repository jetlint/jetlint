package eslintcompat_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jetlint/jetlint/internal/eslintcompat"
)

func TestLoad_ReadsFixtureFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.json")
	body := `{
	  "ruleId": "demo",
	  "oxcSHA": "abc123",
	  "oxcSource": "crates/.../demo.rs",
	  "cases": [
	    {"code": "ok", "valid": true},
	    {"code": "bad", "valid": false}
	  ]
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	fx, err := eslintcompat.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if fx.RuleID != "demo" {
		t.Errorf("RuleID = %q; want demo", fx.RuleID)
	}
	if fx.OxcSHA != "abc123" {
		t.Errorf("OxcSHA = %q; want abc123", fx.OxcSHA)
	}
	if len(fx.Cases) != 2 {
		t.Fatalf("len(Cases) = %d; want 2", len(fx.Cases))
	}
	if !fx.Cases[0].Valid || fx.Cases[0].Code != "ok" {
		t.Errorf("Cases[0] = %#v", fx.Cases[0])
	}
	if fx.Cases[1].Valid || fx.Cases[1].Code != "bad" {
		t.Errorf("Cases[1] = %#v", fx.Cases[1])
	}
}

func TestLoad_ReturnsErrorForMissingFile(t *testing.T) {
	_, err := eslintcompat.Load("/tmp/this-does-not-exist-xyz.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_ReturnsErrorForInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := eslintcompat.Load(path)
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
