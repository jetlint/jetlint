package main

import (
	"fmt"
	"os"
	"path/filepath"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/preferreadonlyparametertypes"
)

const tsconfigBody = `{
  "compilerOptions": {
    "strict": true, "target": "es2022", "module": "esnext",
    "moduleResolution": "bundler", "lib": ["es2022", "dom"],
    "skipLibCheck": true
  },
  "include": ["case.ts"]
}`

func main() {
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(tsconfigBody), 0o644)
	// Exact fixture indentation from tselintcompat_test.go fixture trim.
	os.WriteFile(filepath.Join(dir, "case.ts"), []byte(`
        interface Foo {
          readonly prop: RegExp;
        }

        function foo(arg: Foo) {}
      `), 0o644)
	prog, err := wrapperchecker.LoadProgram(tsc)
	if err != nil { fmt.Println("err:", err); return }
	defer prog.Close()
	opts := preferreadonlyparametertypes.DefaultOptions()
	opts.Allow = []preferreadonlyparametertypes.AllowSpec{{Name: "Bar", From: "file"}}
	eng := engine.New(
		[]engine.Rule{preferreadonlyparametertypes.NewWithOptions(opts)},
		map[string]wrapperlint.Severity{"prefer-readonly-parameter-types": wrapperlint.SeverityError},
	)
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "prefer-readonly-parameter-types" {
			fmt.Printf("Diag: %+v %s\n", d.Range, d.Message)
		}
	}
}
