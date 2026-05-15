package astflow_test

import (
	"os"
	"path/filepath"
	"testing"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/astflow"
)

func classify(t *testing.T, body string) astflow.ReturnStatus {
	t.Helper()
	dir := t.TempDir()
	tsc := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsc, []byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler"}}`), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	src := "const f = function() " + body + ";"
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(src), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}
	defer prog.Close()
	var fn *wrapperchecker.Node
	for _, sf := range prog.SourceFiles() {
		sf.Walk(func(n *wrapperchecker.Node) bool {
			if fn != nil {
				return false
			}
			if n.Kind() == wrapperchecker.KindFunctionExpression {
				fn = n
				return false
			}
			return true
		})
	}
	if fn == nil {
		t.Fatal("no FunctionExpression found")
	}
	return astflow.FunctionBodyReturnStatus(fn)
}

func TestReturnStatus_EmptyBodyIsNotReturn(t *testing.T) {
	if got := classify(t, "{}"); got != astflow.NotReturn {
		t.Errorf("empty body = %v; want NotReturn", got)
	}
}

func TestReturnStatus_ExplicitReturnWithValueIsAlwaysExplicit(t *testing.T) {
	if got := classify(t, "{ return 1; }"); got != astflow.AlwaysExplicit {
		t.Errorf("got = %v; want AlwaysExplicit", got)
	}
}

func TestReturnStatus_ExplicitReturnNoValueIsAlwaysImplicit(t *testing.T) {
	if got := classify(t, "{ return; }"); got != astflow.AlwaysImplicit {
		t.Errorf("got = %v; want AlwaysImplicit", got)
	}
}

func TestReturnStatus_ThrowIsAlwaysExplicit(t *testing.T) {
	if got := classify(t, "{ throw new Error(); }"); got != astflow.AlwaysExplicit {
		t.Errorf("got = %v; want AlwaysExplicit", got)
	}
}

func TestReturnStatus_IfWithBothBranchesReturningValueIsAlwaysExplicit(t *testing.T) {
	got := classify(t, "{ if (1) { return 1; } else { return 2; } }")
	if got != astflow.AlwaysExplicit {
		t.Errorf("got = %v; want AlwaysExplicit", got)
	}
}

func TestReturnStatus_IfWithoutElseLeavesAFallthroughPath(t *testing.T) {
	got := classify(t, "{ if (1) { return 1; } }")
	if got.MustReturn() {
		t.Errorf("got = %v; should not MustReturn (else branch falls through)", got)
	}
	if !got.MayReturnExplicit() {
		t.Errorf("got = %v; should MayReturnExplicit", got)
	}
}

func TestReturnStatus_MixedReturnBranchesProduceMixed(t *testing.T) {
	got := classify(t, "{ if (1) { return 1; } else { return; } }")
	if got != astflow.AlwaysMixed {
		t.Errorf("got = %v; want AlwaysMixed", got)
	}
}

func TestReturnStatus_StatementsAfterReturnAreUnreachable(t *testing.T) {
	got := classify(t, "{ return 1; const x = 2; }")
	if got != astflow.AlwaysExplicit {
		t.Errorf("got = %v; want AlwaysExplicit", got)
	}
}
