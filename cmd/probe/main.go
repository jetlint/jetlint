package main

import (
	"fmt"
	"os"
	"path/filepath"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarycondition"
)

func main() {
	dir, _ := os.MkdirTemp("/tmp", "tsg")
	defer os.RemoveAll(dir)
	tsc := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(tsc, []byte(`{"compilerOptions":{"strict":true,"target":"es2022","module":"esnext","moduleResolution":"bundler","lib":["es2022","dom"],"skipLibCheck":true},"include":["case.ts"]}`), 0o644)
	code, _ := os.ReadFile("/tmp/case0.ts")
	os.WriteFile(filepath.Join(dir, "case.ts"), code, 0o644)
	prog, _ := wrapperchecker.LoadProgram(tsc)
	defer prog.Close()
	eng := engine.New(
		[]engine.Rule{nounnecessarycondition.New()},
		map[string]wrapperlint.Severity{"no-unnecessary-condition": wrapperlint.SeverityError},
	)
	for _, d := range eng.Lint(prog) {
		if d.RuleID == "no-unnecessary-condition" {
			fmt.Printf("line=%d col=%d msg=%s\n", d.Range.StartLine, d.Range.StartColumn, d.Message)
		}
	}
}
