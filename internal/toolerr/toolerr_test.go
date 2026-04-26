package toolerr_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tommymorgan/tsgolint/internal/toolerr"
)

func TestError_ImplementsErrorInterface(t *testing.T) {
	err := toolerr.New(toolerr.CodeInternal, "boom")
	if err.Error() != "boom" {
		t.Errorf("expected message 'boom', got %q", err.Error())
	}
}

func TestWriteJSON_EmitsSingleLineWithCodeAndMessage(t *testing.T) {
	var buf bytes.Buffer
	err := toolerr.WithPath(toolerr.CodeTsconfigMissing, "no tsconfig", "/abs/path")
	if writeErr := err.WriteJSON(&buf); writeErr != nil {
		t.Fatalf("WriteJSON: %v", writeErr)
	}
	out := buf.String()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("expected exactly one newline, got: %q", out)
	}

	var got map[string]any
	if decErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); decErr != nil {
		t.Fatalf("decode: %v\noutput: %s", decErr, out)
	}
	if got["code"] != string(toolerr.CodeTsconfigMissing) {
		t.Errorf("expected code %s, got %v", toolerr.CodeTsconfigMissing, got["code"])
	}
	if got["message"] != "no tsconfig" {
		t.Errorf("expected message 'no tsconfig', got %v", got["message"])
	}
	if got["path"] != "/abs/path" {
		t.Errorf("expected path '/abs/path', got %v", got["path"])
	}
}

func TestWriteJSON_OmitsEmptyPath(t *testing.T) {
	var buf bytes.Buffer
	err := toolerr.New(toolerr.CodeInternal, "boom")
	if writeErr := err.WriteJSON(&buf); writeErr != nil {
		t.Fatalf("WriteJSON: %v", writeErr)
	}
	if strings.Contains(buf.String(), `"path"`) {
		t.Errorf("expected path field to be omitted, got: %s", buf.String())
	}
}

func TestNewf_FormatsMessage(t *testing.T) {
	err := toolerr.Newf(toolerr.CodeConfigInvalid, "bad %s on line %d", "syntax", 7)
	if err.Message != "bad syntax on line 7" {
		t.Errorf("got %q", err.Message)
	}
}
