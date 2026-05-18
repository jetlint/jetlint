package nounnecessarytypeassertion_test

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeassertion"
)

// Regression for jetlint#624: a real-world Remix codebase blew the
// goroutine stack past 1GB inside the rule's `typeContains` walk. The
// seen-set keyed on wrapper `*Type` pointers but UnionMembers() and
// TypeArguments() return fresh wrappers around the same underlying
// checker type — so cycle detection never engaged on recursive shapes.
// This test loads a tsconfig that uses the DOM lib (the easiest way to
// drag in the deeply cross-referenced lib.dom.d.ts types) and runs the
// rule on a small fixture exercising the cast patterns that crashed.
// The fix collapses identical underlying types in the seen-set, so the
// walk terminates; with a regression, it would not terminate within
// the 30s test deadline at the stack budget set below.
func TestNoUnnecessaryTypeAssertion_TerminatesOnDeeplyCrossReferencedDomLibTypes(t *testing.T) {
	debug.SetMaxStack(8 * 1024 * 1024)

	dir := t.TempDir()
	tsconfig := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsconfig, []byte(`{"compilerOptions":{"jsx":"react-jsx","strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler","lib":["es2022","dom","dom.iterable"]}}`), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	code := `
declare const anyEl: any;
const div = anyEl as HTMLDivElement;
const input = anyEl as HTMLInputElement;
const ev = anyEl as MouseEvent;
const target = (ev.target as HTMLElement);
const ke = ev as KeyboardEvent;
const promiseEl = anyEl as Promise<HTMLElement>;
const mapEl = anyEl as Record<string, HTMLElement>;
const elArr = anyEl as HTMLElement[];
const nested = anyEl as Promise<HTMLElement[]>;
`
	if err := os.WriteFile(filepath.Join(dir, "case.ts"), []byte(code), 0o644); err != nil {
		t.Fatalf("write case: %v", err)
	}

	done := make(chan struct{})
	go func() {
		prog, err := wrapperchecker.LoadProgram(tsconfig)
		if err != nil {
			t.Errorf("LoadProgram: %v", err)
			close(done)
			return
		}
		defer prog.Close()
		eng := engine.New(
			[]engine.Rule{nounnecessarytypeassertion.New()},
			map[string]wrapperlint.Severity{"no-unnecessary-type-assertion": wrapperlint.SeverityError},
		)
		_ = eng.Lint(prog)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("rule did not terminate in 30s on dom-lib fixture — typeContains is recursing without cycle detection")
	}
}
