package load

import (
	"bytes"
	"fmt"
	"go/printer"
	"path/filepath"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/stdlib"
)

// TestMacroWalkSkipped checks that leaving out the expansion of the built-in
// macros for a package without macro syntax (hasMacroSyntax) changes nothing:
// over the programs of the repository (the examples, tests/run and tests/errors)
// the syntax trees after the load, printed, and the load errors are the same
// with and without the walk.
func TestMacroWalkSkipped(t *testing.T) {
	type program struct{ name, dir, pattern string }
	var programs []program
	examples, _ := filepath.Glob("../../examples/*/main.gp")
	for _, m := range examples {
		dir, err := filepath.Abs(filepath.Dir(m))
		if err != nil {
			t.Fatal(err)
		}
		programs = append(programs, program{"examples/" + filepath.Base(dir), dir, "."})
	}
	for _, folder := range []string{"run", "errors"} {
		dir, err := filepath.Abs("../../tests/" + folder)
		if err != nil {
			t.Fatal(err)
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.gp"))
		for _, f := range files {
			programs = append(programs, program{folder + "/" + filepath.Base(f), dir, f})
		}
	}
	if len(programs) < 100 {
		t.Skipf("only %d programs: the repository's tests are not here", len(programs))
	}
	dump := func(pr program) string {
		cfg := Config{Dir: pr.dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
		prog := Load(cfg, pr.pattern)
		var b bytes.Buffer
		for _, p := range prog.Packages {
			if p.Std {
				continue
			}
			fmt.Fprintf(&b, "package %s\n", p.ImportPath)
			for _, f := range p.Files {
				fmt.Fprintf(&b, "file %s\n", f.Path)
				printer.Fprint(&b, prog.Fset, f.AST)
				b.WriteString("\n")
			}
			(&diag.Printer{All: true}).Print(&b, p.Errors)
		}
		return b.String()
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	skipped := 0
	// forceMacroWalk is global: run the whole comparison once per setting.
	results := make([]string, len(programs))
	forceMacroWalk = false
	for i, pr := range programs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = dump(pr)
		}()
	}
	wg.Wait()
	forceMacroWalk = true
	defer func() { forceMacroWalk = false }()
	for i, pr := range programs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			got := dump(pr)
			mu.Lock()
			defer mu.Unlock()
			if got != results[i] {
				t.Errorf("%s: the load differs with the walk over a package without macro syntax", pr.name)
			}
			skipped++
		}()
	}
	wg.Wait()
	if skipped < 100 {
		t.Errorf("only %d programs compared", skipped)
	}
}
