package vet

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lower"
)

// A dependency that is not a root: its own `!` is not reported (the user vets their package, not
// the library they use), but the call of it in the root is: the fact travels with the package.
func TestPanicbangDependencyNotRoot(t *testing.T) {
	p := loadProgram(t, map[string]string{
		"dep/dep.gp": `package dep

import "strconv"

func parse(s string) int {
	return strconv.Atoi!(s)
}

func Parse(s string) int { return parse(s) }

func Fine(s string) (int, error) { return strconv.Atoi(s) }
`,
		"lib/lib.gp": `package lib

import "example.com/m/dep"

func Port(s string) int {
	return dep.Parse(s)
}

func Fine(s string) (int, error) { return dep.Fine(s) }
`,
		"app/main.gp": `package main

import "example.com/m/dep"

func main() { println(dep.Parse("1")) }
`,
	}, "./lib", "./app", "./dep")
	// Only lib and app are roots.
	p.prog.Roots = p.prog.Roots[:0]
	for _, lp := range p.prog.Packages {
		if strings.HasSuffix(lp.ImportPath, "/lib") || strings.HasSuffix(lp.ImportPath, "/app") {
			p.prog.Roots = append(p.prog.Roots, lp)
		}
	}
	if len(p.prog.Roots) != 2 {
		t.Fatalf("roots: %v", p.prog.Roots)
	}
	rep := p.vet(t, "panicbang")
	if len(rep.Findings) != 1 {
		t.Fatalf("want one finding, got %+v", rep.Findings)
	}
	f := rep.Findings[0]
	if !strings.HasSuffix(f.Package, "/lib") || f.Pos.Line != 6 || !strings.Contains(f.Message, "dep.Parse can panic: dep.parse uses ! at dep.gp:6:9") {
		t.Errorf("finding: %s: %s", p.short(f.Pos), f.Message)
	}
}

// The analyzers of the Phase 10 forms run in a package with C/C++ names (the guard does not skip
// them): the facts of a `!` and the lazy parameters are found next to opaque values.
func TestPhase10AnalyzersWithCNames(t *testing.T) {
	p := loadProgram(t, map[string]string{
		"main.gp": `package main

import (
	"<cstdio>"
	"os"
)

macro Skip(a lazy int, b lazy int) { // want ` + "`the lazy parameter b of macro Skip is never used`" + `
	cstdio.printf("%d\n", C.int(a()))
}

func clean!(p string) {
	go func() error { // want ` + "`go func\\(\\) error`" + `
		os.Remove!(p)
		return nil
	}()
}

func main() {
	#Skip(1, 2)
	clean!("x")
}
`,
	})
	rep := p.vet(t, "gobang", "unusedlazy", "unwaitedbody", "panicbang", "ignorederr")
	p.expect(t, rep, "")
}

// TestMustCorpus vets the programs of the language tests for `!` and macros: no analyzer of
// gocpp reports anything but what the program shows on purpose.
func TestMustCorpus(t *testing.T) {
	files := corpusFiles(t, "tests/run/must_*.gp", "tests/run/umacro_*.gp", "tests/golden/macros_user.gp", "examples/must-errors/main.gp")
	vetted := 0
	for _, f := range files {
		rep, ok := sweepFile(t, f, false)
		if !ok {
			continue
		}
		vetted++
		for _, c := range rep.Crashed() {
			t.Errorf("%s: analyzer %s crashed: %s", f, c.Analyzer, strings.SplitN(c.Text, "\n", 2)[0])
		}
		for _, fd := range rep.Findings {
			switch fd.Analyzer {
			case "panicbang", "gobang", "unwaitedbody", "unusedlazy", "ignorederr", "cpplocal", "cstr", "narrowing":
				if !knownFindings[filepath.Base(fd.Pos.Filename)+" "+fd.Analyzer] {
					t.Errorf("%s:%d:%d: unexpected finding: [%s] %s", filepath.Base(fd.Pos.Filename), fd.Pos.Line, fd.Pos.Column, fd.Analyzer, fd.Message)
				}
			}
		}
	}
	if vetted < 130 {
		t.Errorf("only %d of %d files could be vetted", vetted, len(files))
	}
}

// go/ssa crashes on a conversion of a slice to an array type that a type parameter gives
// (x/tools v0.44). nilness builds again without the generic functions, finds what it can in the
// rest, and says so in the notes: no crash, no exit status 2.
func TestNilnessGenericSliceToArray(t *testing.T) {
	p := loadProgram(t, map[string]string{
		"main.gp": `package main

import "fmt"

func Arr[S ~[]E, A ~[3]E, E any](s S) A { return A(s) }

type Box[T any] struct{ v T }

func (b *Box[T]) Get() T { return b.v }

type P struct{ x int }

func deref() int {
	var p *P
	if p == nil { // want ` + "`tautological condition: nil == nil`" + `
		return p.x // want ` + "`nil dereference in field selection`" + `
	}
	return 0
}

func main() {
	a := Arr[[]int, [3]int]([]int{1, 2, 3})
	b := &Box[int]{1}
	fmt.Println(a, b.Get(), deref())
}
`,
	})
	rep := p.vet(t, "nilness")
	p.expect(t, rep, "")
	found := false
	for _, n := range rep.Notes {
		if n.Analyzer == "nilness" && n.Skipped && strings.Contains(n.Text, "go/ssa crashed") {
			found = true
		}
	}
	if !found {
		t.Errorf("no note about the generic functions: %+v", rep.Notes)
	}
}

// The cstr analyzer keeps a copy of the "keeps the pointer" tables of the lowering (keeps.go): the
// two must be the same.
func TestKeepsTablesMatchLowering(t *testing.T) {
	if !reflect.DeepEqual(keeperFuncs, lower.KeeperFuncs()) {
		t.Errorf("keeperFuncs differ:\nvet   %v\nlower %v", keeperFuncs, lower.KeeperFuncs())
	}
	if !reflect.DeepEqual(destructorArgs, lower.DestructorArgs()) {
		t.Errorf("destructorArgs differ:\nvet   %v\nlower %v", destructorArgs, lower.DestructorArgs())
	}
	if !reflect.DeepEqual(conventionWords, lower.ConventionWords()) {
		t.Errorf("conventionWords differ:\nvet   %v\nlower %v", conventionWords, lower.ConventionWords())
	}
}

// The same crash comes from a method of a generic type: pruning leaves out those too.
func TestNilnessGenericMethodSliceToArray(t *testing.T) {
	p := loadProgram(t, map[string]string{
		"main.gp": `package main

import "fmt"

type B[S ~[]E, A ~[3]E, E any] struct{}

func (B[S, A, E]) Conv(s S) A { return A(s) }

func main() {
	var b B[[]int, [3]int, int]
	fmt.Println(b.Conv([]int{1, 2, 3}))
}
`,
	})
	rep := p.vet(t, "nilness")
	p.expect(t, rep, "")
	found := false
	for _, n := range rep.Notes {
		if n.Analyzer == "nilness" && n.Skipped && strings.Contains(n.Text, "go/ssa crashed") {
			found = true
		}
	}
	if !found {
		t.Errorf("no note about the generic functions: %+v", rep.Notes)
	}
}
