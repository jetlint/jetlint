package nofloatingpromises_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tommymorgan/tsgolint/internal/rules/nofloatingpromises"
)

func TestOptionsFromJSON_EmptyInputYieldsDefaults(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage("null")} {
		got, err := nofloatingpromises.OptionsFromJSON(raw)
		if err != nil {
			t.Fatalf("unexpected error for raw %q: %v", raw, err)
		}
		if !reflect.DeepEqual(got, nofloatingpromises.DefaultOptions()) {
			t.Errorf("raw %q: expected defaults, got %#v", raw, got)
		}
	}
}

func TestOptionsFromJSON_OverridesEachField(t *testing.T) {
	raw := json.RawMessage(`{
		"ignoreVoid": false,
		"ignoreIIFE": true,
		"checkThenables": true,
		"allowForKnownSafePromises": [{"from": "file", "name": "Foo"}, "Bar"],
		"allowForKnownSafeCalls": ["log"]
	}`)
	got, err := nofloatingpromises.OptionsFromJSON(raw)
	if err != nil {
		t.Fatalf("OptionsFromJSON: %v", err)
	}
	want := nofloatingpromises.Options{
		IgnoreVoid:     false,
		IgnoreIIFE:     true,
		CheckThenables: true,
		AllowForKnownSafePromises: []nofloatingpromises.TypeMatcher{
			{From: "file", Name: "Foo"},
			{Name: "Bar"},
		},
		AllowForKnownSafeCalls: []nofloatingpromises.TypeMatcher{
			{Name: "log"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestOptionsFromJSON_RejectsUnknownKey(t *testing.T) {
	raw := json.RawMessage(`{"unknownOpt": true}`)
	_, err := nofloatingpromises.OptionsFromJSON(raw)
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestOptionsFromJSON_RejectsBadShape(t *testing.T) {
	raw := json.RawMessage(`"not-an-object"`)
	_, err := nofloatingpromises.OptionsFromJSON(raw)
	if err == nil {
		t.Fatal("expected error for non-object input, got nil")
	}
}

func TestOptionsFromJSON_RejectsBadMatcherShape(t *testing.T) {
	raw := json.RawMessage(`{"allowForKnownSafePromises": [42]}`)
	_, err := nofloatingpromises.OptionsFromJSON(raw)
	if err == nil {
		t.Fatal("expected error for numeric matcher, got nil")
	}
}

func TestOptionsFromJSON_RejectsBadFieldType(t *testing.T) {
	raw := json.RawMessage(`{"ignoreVoid": "yes"}`)
	_, err := nofloatingpromises.OptionsFromJSON(raw)
	if err == nil {
		t.Fatal("expected error for non-bool ignoreVoid, got nil")
	}
}
