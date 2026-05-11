package cli_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/jetlint/jetlint/internal/cli"
)

func TestVersion_PrintsSingleLineSemanticVersionAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := cli.Run([]string{"--version"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("expected exit 0, got %d. stderr:\n%s", exit, stderr.String())
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single line of output, got %d lines:\n%s", len(lines), stdout.String())
	}
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	if !semver.MatchString(strings.TrimSpace(lines[0])) {
		t.Errorf("output %q does not match semantic version shape", lines[0])
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}
}

func TestHelp_PrintsUsageAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := cli.Run([]string{"--help"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("expected exit 0, got %d. stderr:\n%s", exit, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Usage", "jetlint", "--version", "--help", "Exit codes"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage output missing %q. got:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}
}

func TestUnknownFlag_ExitsTwoWithErrorOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := cli.Run([]string{"--bogus"}, &stdout, &stderr)
	if exit != 2 {
		t.Errorf("expected exit 2 for unknown flag, got %d", exit)
	}
	if stderr.Len() == 0 {
		t.Errorf("expected diagnostic on stderr for unknown flag, got empty")
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on parse error, got: %s", stdout.String())
	}
}
