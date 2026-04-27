package tselintcompat_test

import (
	"path/filepath"
	"testing"

	"github.com/tommymorgan/tsgolint/internal/tselintcompat"
)

func TestLoad_NoFloatingPromisesFixtureExtractsCases(t *testing.T) {
	path, err := filepath.Abs("../../testdata/typescript-eslint/no-floating-promises.test.ts")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	cases, err := tselintcompat.Load(path, "no-floating-promises")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("expected at least one case extracted")
	}

	var valid, invalid, withOpts, withErrors int
	for _, c := range cases {
		if c.Valid {
			valid++
		} else {
			invalid++
			if c.ExpectedErrorCount > 0 {
				withErrors++
			}
		}
		if c.HasOptions {
			withOpts++
		}
		if c.Code == "" {
			t.Errorf("case %d (valid=%v) has empty code", c.SourceIndex, c.Valid)
		}
	}

	t.Logf("extracted %d cases (%d valid, %d invalid, %d with options, %d invalid-with-errors-count)",
		len(cases), valid, invalid, withOpts, withErrors)
	if valid == 0 || invalid == 0 {
		t.Errorf("expected both valid and invalid cases, got valid=%d invalid=%d", valid, invalid)
	}
}
