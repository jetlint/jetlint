package engine_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
)

// panicRule is a test-only rule whose handler panics on every visit.
// It exists to validate the engine's panic-recovery contract.
type panicRule struct {
	id   string
	kind wrapperchecker.Kind
}

func (r panicRule) Meta() engine.Meta { return engine.Meta{ID: r.id} }
func (r panicRule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		r.kind: func(_ *engine.Context, _ *wrapperchecker.Node) {
			panic("kaboom")
		},
	}
}

// quietRule reports a diagnostic on every variable declaration so the
// test can assert non-panicking rules continue to fire even when a
// sibling rule blows up.
type quietRule struct{ id string }

func (r quietRule) Meta() engine.Meta { return engine.Meta{ID: r.id} }
func (r quietRule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: func(ctx *engine.Context, n *wrapperchecker.Node) {
			ctx.Report(n, "saw a variable declaration")
		},
	}
}

func loadTinyProgram(t *testing.T) *wrapperchecker.Program {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`),
		0o644); err != nil {
		t.Fatalf("tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte("const x = 1;\n"), 0o644); err != nil {
		t.Fatalf("main.ts: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)
	return prog
}

func TestEngine_PanicInRuleBecomesPerFileToolErrorAndDoesNotAbortLint(t *testing.T) {
	prog := loadTinyProgram(t)
	eng := engine.New(
		[]engine.Rule{
			panicRule{id: "test/panic", kind: wrapperchecker.KindVariableDeclaration},
			quietRule{id: "test/quiet"},
		},
		map[string]wrapperlint.Severity{
			"test/panic": wrapperlint.SeverityError,
			"test/quiet": wrapperlint.SeverityError,
		},
	)
	diags := eng.Lint(prog)

	// Expect both a panic-reported diagnostic and the quiet rule's diagnostic.
	hasPanic, hasQuiet := false, false
	for _, d := range diags {
		if strings.Contains(d.RuleID, "rule-panic") {
			hasPanic = true
		}
		if d.RuleID == "test/quiet" {
			hasQuiet = true
		}
	}
	if !hasPanic {
		t.Errorf("expected a jetlint/rule-panic diagnostic, got: %#v", diags)
	}
	if !hasQuiet {
		t.Errorf("expected the non-panicking rule to still fire, got: %#v", diags)
	}
}
