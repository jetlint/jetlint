package nofloatingpromises_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nofloatingpromises"

	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// fixture writes a small TypeScript project to a temp dir and returns
// the absolute tsconfig path.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tsg")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	tsconfig := filepath.Join(dir, "tsconfig.json")
	if _, hasConfig := files["tsconfig.json"]; !hasConfig {
		files["tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return tsconfig
}

func runRule(t *testing.T, tsconfig string) []wrapperlint.Diagnostic {
	t.Helper()
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	t.Cleanup(prog.Close)

	eng := engine.New(
		[]engine.Rule{nofloatingpromises.New()},
		map[string]wrapperlint.Severity{"no-floating-promises": wrapperlint.SeverityError},
	)
	return eng.Lint(prog)
}

func TestNoFloatingPromises_FlagsUnawaitedPromiseFromImportedFunction(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"api.ts":  "export async function saveUser(name: string): Promise<void> { return; }\n",
		"main.ts": "import { saveUser } from \"./api\";\nsaveUser(\"alice\");\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	d := diags[0]
	if !strings.HasSuffix(d.Range.File, "main.ts") {
		t.Errorf("expected diagnostic in main.ts, got %s", d.Range.File)
	}
	if d.RuleID != "no-floating-promises" {
		t.Errorf("expected rule id 'no-floating-promises', got %q", d.RuleID)
	}
	if d.Range.StartLine != 2 {
		t.Errorf("expected line 2, got %d", d.Range.StartLine)
	}
}

func TestNoFloatingPromises_DoesNotFlagAwaitedPromise(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"api.ts":  "export async function fetchUser(): Promise<string> { return \"x\"; }\n",
		"main.ts": "import { fetchUser } from \"./api\";\nasync function run() { await fetchUser(); }\nrun();\n",
	})
	diags := runRule(t, tsconfig)
	// run() itself is also a promise; it returns from an async function.
	// We expect one diagnostic on `run();` (line 3) but NOT on `await fetchUser();`.
	for _, d := range diags {
		if strings.Contains(d.Range.File, "main.ts") && d.Range.StartLine == 2 {
			t.Errorf("did not expect diagnostic on awaited call, got: %#v", d)
		}
	}
}

func TestNoFloatingPromises_DoesNotFlagPromiseExplicitlyVoided(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"api.ts":  "export async function fireAndForget(): Promise<void> { return; }\n",
		"main.ts": "import { fireAndForget } from \"./api\";\nvoid fireAndForget();\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for void-prefixed promise, got %d: %#v", len(diags), diags)
	}
}

func TestNoFloatingPromises_DoesNotFlagPromiseReturnedFromArrowImplicitBody(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"api.ts":  "export async function fetchUser(): Promise<string> { return \"x\"; }\n",
		"main.ts": "import { fetchUser } from \"./api\";\nconst handler = () => fetchUser();\n",
	})
	diags := runRule(t, tsconfig)
	// The arrow function's concise body IS the promise expression. The
	// rule must not flag it: returning the promise from the arrow
	// hands handling responsibility to whoever calls handler().
	for _, d := range diags {
		if strings.Contains(d.Range.File, "main.ts") && d.Range.StartLine == 2 {
			t.Errorf("did not expect diagnostic on arrow concise body, got: %#v", d)
		}
	}
}

// Regression: a Promise-returning call inside a void-typed callback used to
// be missed in real codebases because contextual narrowing can hide the
// Promise from GetTypeAtLocation. The fix consults the resolved call
// signature's declared return type as a second signal, which catches the
// `Promise<T> | void` callback shape (common in react-testing-library's
// `act` and many plain `() => Promise<void> | void` consumer APIs).
//
// Known limitation: `unknown | Promise<unknown>` collapses to `unknown`
// at the type-system level (because Promise<T> is assignable to unknown),
// so neither the contextual type nor the signature return type carries
// the Promise information. Catching that variant — used by
// react-hook-form's SubmitHandler — would require walking the called
// symbol's declared return-type AST, which is deferred.
func TestNoFloatingPromises_FlagsPromiseCallInsideVoidOrPromiseCallback(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "" +
			"type CB = (data: object) => Promise<void> | void;\n" +
			"declare const cb: CB;\n" +
			"declare function handleSubmit(callback: CB): void;\n" +
			"handleSubmit((data) => {\n" +
			"  cb(data);\n" +
			"});\n",
	})
	diags := runRule(t, tsconfig)
	found := false
	for _, d := range diags {
		if strings.HasSuffix(d.Range.File, "main.ts") && d.Range.StartLine == 5 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a no-floating-promises diagnostic on the cb() call inside the Promise<void>|void callback, got: %#v", diags)
	}
}

func TestNoFloatingPromises_FlagsPromiseCallInsideVoidCallback(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "" +
			"declare function handleSubmit(cb: (data: object) => void): void;\n" +
			"declare function onSubmit(data: object): Promise<void>;\n" +
			"handleSubmit((data) => {\n" +
			"  onSubmit(data);\n" +
			"});\n",
	})
	diags := runRule(t, tsconfig)
	found := false
	for _, d := range diags {
		if strings.HasSuffix(d.Range.File, "main.ts") && d.Range.StartLine == 4 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a no-floating-promises diagnostic on the onSubmit call inside the void-typed callback, got: %#v", diags)
	}
}

func TestNoFloatingPromises_DoesNotFlagSynchronousFunctionCall(t *testing.T) {
	tsconfig := fixture(t, map[string]string{
		"main.ts": "function add(a: number, b: number): number { return a + b; }\nadd(1, 2);\n",
	})
	diags := runRule(t, tsconfig)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for sync function call, got %d: %#v", len(diags), diags)
	}
}
