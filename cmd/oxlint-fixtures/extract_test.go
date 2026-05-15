package main

import "testing"

func TestExtract_PullsPlainStringEntries(t *testing.T) {
	src := `
fn test() {
    let pass = vec!["x === y", "1 === 2"];
    let fail = vec!["x === x", "x !== x"];
}
`
	pass, fail, err := Extract(src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(pass) != 2 || pass[0].Code != "x === y" || pass[1].Code != "1 === 2" {
		t.Errorf("pass = %#v", pass)
	}
	if len(fail) != 2 || fail[0].Code != "x === x" || fail[1].Code != "x !== x" {
		t.Errorf("fail = %#v", fail)
	}
}

func TestExtract_PullsCodeFromTupleEntries(t *testing.T) {
	src := `
fn test() {
    let pass = vec![
        ("foo.indexOf(NaN)", Some(serde_json::json!([{ "enforceForIndexOf": true }]))),
        "bareValid",
    ];
    let fail = vec![("x === x", None), ("y !== y", None)];
}
`
	pass, fail, err := Extract(src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(pass) != 2 || pass[0].Code != "foo.indexOf(NaN)" || pass[1].Code != "bareValid" {
		t.Errorf("pass = %#v", pass)
	}
	if !pass[0].HasOptions {
		t.Errorf("pass[0].HasOptions should be true (Some(...) tuple)")
	}
	if pass[1].HasOptions {
		t.Errorf("pass[1].HasOptions should be false (bare string)")
	}
	if len(fail) != 2 || fail[0].Code != "x === x" || fail[1].Code != "y !== y" {
		t.Errorf("fail = %#v", fail)
	}
	if fail[0].HasOptions || fail[1].HasOptions {
		t.Errorf("None-option tuples should not be flagged as having options")
	}
}

func TestExtract_HandlesRawStrings(t *testing.T) {
	src := `
fn test() {
    let pass = vec![
        r"plain raw content",
        r#"content with "embedded quotes""#,
        r##"can embed "# inside if outer uses ##"##,
    ];
    let fail = vec![];
}
`
	pass, _, err := Extract(src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(pass) != 3 {
		t.Fatalf("expected 3 pass entries, got %d: %#v", len(pass), pass)
	}
	if pass[0].Code != `plain raw content` {
		t.Errorf("raw string content wrong: %q", pass[0].Code)
	}
	if pass[1].Code != `content with "embedded quotes"` {
		t.Errorf("hashed raw string content wrong: %q", pass[1].Code)
	}
	if pass[2].Code != `can embed "# inside if outer uses ##` {
		t.Errorf("double-hashed raw string content wrong: %q", pass[2].Code)
	}
}

func TestExtract_StringEscapesDecodedCorrectly(t *testing.T) {
	src := `
fn test() {
    let pass = vec!["line1\nline2", "tab\there", "quote\"end"];
    let fail = vec![];
}
`
	pass, _, err := Extract(src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(pass) != 3 {
		t.Fatalf("expected 3, got %d: %#v", len(pass), pass)
	}
	if pass[0].Code != "line1\nline2" {
		t.Errorf("\\n decode: %q", pass[0].Code)
	}
	if pass[1].Code != "tab\there" {
		t.Errorf("\\t decode: %q", pass[1].Code)
	}
	if pass[2].Code != `quote"end` {
		t.Errorf("\\\" decode: %q", pass[2].Code)
	}
}

func TestExtract_IgnoresCommentsInsideVec(t *testing.T) {
	src := `
fn test() {
    let pass = vec![
        // a comment with "fake" string
        "real entry",
        /* block comment "also fake" */
        "real entry 2",
    ];
    let fail = vec![];
}
`
	pass, _, err := Extract(src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(pass) != 2 || pass[0].Code != "real entry" || pass[1].Code != "real entry 2" {
		t.Errorf("pass = %#v", pass)
	}
}

func TestExtract_HandlesNestedBracketsInEntries(t *testing.T) {
	src := `
fn test() {
    let fail = vec![
        ("foo[0] === foo[0]", None),
        "obj.bar([1, 2]) === obj.bar([1, 2])",
    ];
}
`
	_, fail, err := Extract(src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(fail) != 2 {
		t.Fatalf("expected 2 fail, got %d: %#v", len(fail), fail)
	}
	if fail[0].Code != "foo[0] === foo[0]" {
		t.Errorf("first: %q", fail[0].Code)
	}
	if fail[1].Code != "obj.bar([1, 2]) === obj.bar([1, 2])" {
		t.Errorf("second: %q", fail[1].Code)
	}
}

func TestExtract_ReturnsEmptyForMissingVec(t *testing.T) {
	src := `
fn test() {
    let pass = vec!["only pass"];
    // no fail vec
}
`
	pass, fail, err := Extract(src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(pass) != 1 || pass[0].Code != "only pass" {
		t.Errorf("pass = %#v", pass)
	}
	if fail != nil {
		t.Errorf("expected fail nil, got %#v", fail)
	}
}
