package vet

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// A program with findings in several analyzers and packages.
func manyFindings() map[string]string {
	return map[string]string{
		"a/a.gp": `package a

import "fmt"

func Logf(format string, args ...any) { fmt.Println(fmt.Sprintf(format, args...)) }

func Bad() {
	fmt.Printf("%d\n", "x")
	var f func()
	f = func() { f() }
	f()
}
`,
		"b/b.gp": `package b

import (
	"fmt"

	"example.com/m/a"
)

func Bad2() {
	a.Logf("%s", 1)
	fmt.Println("x %d")
	x := 1
	x = x
}
`,
		"main.gp": `package main

import (
	"example.com/m/a"
	"example.com/m/b"
)

func _Reserved() {}

func main() { a.Bad(); b.Bad2() }
`,
	}
}

func TestDeterministicOutput(t *testing.T) {
	p := loadProgram(t, manyFindings(), ".", "./a", "./b")
	first := p.vet(t)
	if len(first.Findings) < 6 {
		t.Fatalf("want at least 6 findings, got %d: %+v", len(first.Findings), first.Findings)
	}
	for i := 0; i < 20; i++ {
		again := p.vet(t)
		if !reflect.DeepEqual(first.Findings, again.Findings) {
			t.Fatalf("run %d differs:\n%+v\n%+v", i, first.Findings, again.Findings)
		}
	}
	// Sorted: by package (the order of the roots), then by position.
	for i := 1; i < len(first.Findings); i++ {
		a, b := first.Findings[i-1], first.Findings[i]
		if a.Package == b.Package && (a.Pos.Filename > b.Pos.Filename || a.Pos.Filename == b.Pos.Filename && a.Pos.Line > b.Pos.Line) {
			t.Errorf("findings %d and %d are out of order: %v, %v", i-1, i, a.Pos, b.Pos)
		}
	}
}

func TestRootsOnly(t *testing.T) {
	// Only the root is reported: a and b have findings, but they are
	// imported, not roots.
	p := loadProgram(t, manyFindings(), ".")
	rep := p.vet(t)
	for _, f := range rep.Findings {
		if f.Package != "command-line-arguments" && f.Package != "example.com/m" {
			t.Errorf("finding in %s, which is not a root: %s", f.Package, f.Message)
		}
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Analyzer != "reserved" {
		t.Errorf("want the reserved name only, got %+v", rep.Findings)
	}
}

func TestSelect(t *testing.T) {
	all := Select(nil, nil)
	if len(all) != len(Analyzers) {
		t.Errorf("Select(nil, nil) has %d analyzers, want %d", len(all), len(Analyzers))
	}
	only := Select(map[string]bool{"printf": true, "cycle": true}, nil)
	var names []string
	for _, a := range only {
		names = append(names, a.Name)
	}
	if got := strings.Join(names, ","); got != "printf,cycle" {
		t.Errorf("on = printf,cycle gave %s", got)
	}
	off := Select(nil, map[string]bool{"printf": true})
	if len(off) != len(Analyzers)-1 {
		t.Errorf("off = printf left %d analyzers", len(off))
	}
	for _, a := range off {
		if a.Name == "printf" {
			t.Error("printf is still there")
		}
	}
	if Lookup("nonesuch") != nil || Lookup("cycle") == nil {
		t.Error("Lookup is wrong")
	}
}

func TestAnalyzerNamesUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, a := range Analyzers {
		if seen[a.Name] {
			t.Errorf("two analyzers are named %s", a.Name)
		}
		seen[a.Name] = true
		if a.Doc == "" {
			t.Errorf("analyzer %s has no Doc", a.Name)
		}
	}
	if err := analysis.Validate(func() []*analysis.Analyzer {
		var as []*analysis.Analyzer
		for _, a := range Analyzers {
			as = append(as, a.Analyzer)
		}
		return as
	}()); err != nil {
		t.Error(err)
	}
}

// panicker is an analyzer that crashes; a crash must be a note, not a
// crash of vet.
var panicker = &Analyzer{Analyzer: &analysis.Analyzer{
	Name: "panicker",
	Doc:  "panics",
	Run:  func(*analysis.Pass) (any, error) { panic("boom") },
}}

var failer = &Analyzer{Analyzer: &analysis.Analyzer{
	Name: "failer",
	Doc:  "fails",
	Run:  func(*analysis.Pass) (any, error) { return nil, fmt.Errorf("it failed") },
}}

func TestAnalyzerCrashIsANote(t *testing.T) {
	p := loadProgram(t, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"})
	rep, err := Run(p.prog, p.res, Config{Roots: p.prog.Roots, Analyzers: []*Analyzer{panicker, failer, Lookup("cycle")}})
	if err != nil {
		t.Fatal(err)
	}
	crashed := rep.Crashed()
	if len(crashed) != 2 {
		t.Fatalf("want 2 crash notes, got %+v", rep.Notes)
	}
	if crashed[0].Analyzer != "failer" || !strings.Contains(crashed[0].Text, "it failed") {
		t.Errorf("note 0: %+v", crashed[0])
	}
	if crashed[1].Analyzer != "panicker" || !strings.Contains(crashed[1].Text, "boom") {
		t.Errorf("note 1: %+v", crashed[1])
	}
}

func TestSkippedWhenOpaque(t *testing.T) {
	// An analyzer that doesn't declare RunDespiteErrors and isn't Opaque
	// doesn't run in a package with C++ names; it says so for a root.
	ran := false
	strict := &Analyzer{Analyzer: &analysis.Analyzer{
		Name: "strict", Doc: "needs complete types",
		Run: func(*analysis.Pass) (any, error) { ran = true; return nil, nil },
	}}
	p := loadProgram(t, map[string]string{"main.gp": "package main\n\nimport \"<vector>\"\n\nfunc main() { v := std.vector[int]{}; _ = v.size() }\n"})
	rep, err := Run(p.prog, p.res, Config{Roots: p.prog.Roots, Analyzers: []*Analyzer{strict}})
	if err != nil {
		t.Fatal(err)
	}
	if ran {
		t.Error("the strict analyzer ran in a package with C++ names")
	}
	if len(rep.Notes) != 1 || !rep.Notes[0].Skipped || !strings.Contains(rep.Notes[0].Text, "C/C++") {
		t.Errorf("want one skip note, got %+v", rep.Notes)
	}
	// In a package without C++ names it runs.
	p = loadProgram(t, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"})
	if _, err := Run(p.prog, p.res, Config{Roots: p.prog.Roots, Analyzers: []*Analyzer{strict}}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("the strict analyzer did not run in a plain package")
	}
}

// TestPruneNotes: what pruning leaves out is told in notes (for -v), the
// pruned copies are not kept after Run, and an analyzer that was not
// selected has no note.
func TestPruneNotes(t *testing.T) {
	src := "package main\n\nimport (\n\t\"fmt\"\n\n\t\"<vector>\"\n)\n\nfunc main() {\n\tv := std.vector[int]{}\n\tfmt.Println(v.size())\n\tfmt.Printf(\"%d\\n\", v.size())\n}\n"
	p := loadProgram(t, map[string]string{"main.gp": src})
	rep := p.vet(t, "printf", "nilness")
	got := map[string]Note{}
	for _, n := range rep.Notes {
		got[n.Analyzer] = n
	}
	if n := got["nilness"]; !n.Skipped || !strings.Contains(n.Text, "1 function left out") || len(n.Where) != 1 || n.Where[0].Line != 9 {
		t.Errorf("nilness note: %+v", n)
	}
	if n := got["printf"]; !n.Skipped || !strings.Contains(n.Text, "1 statement left out") || !strings.Contains(n.Text, "2 statements checked") || len(n.Where) != 1 || n.Where[0].Line != 10 {
		t.Errorf("printf note: %+v", n)
	}
	if len(got) != 2 {
		t.Errorf("notes: %+v", rep.Notes)
	}
	prunedMu.Lock()
	kept := len(prunedBy)
	prunedMu.Unlock()
	if kept != 0 {
		t.Errorf("%d pruned copies kept after Run", kept)
	}
	// Only printf selected: no note about nilness.
	rep = p.vet(t, "printf")
	for _, n := range rep.Notes {
		if n.Analyzer != "printf" {
			t.Errorf("note of an analyzer that did not run: %+v", n)
		}
	}
}

func TestNoAnalyzers(t *testing.T) {
	p := loadProgram(t, manyFindings(), ".")
	rep, err := Run(p.prog, p.res, Config{Roots: p.prog.Roots})
	if err != nil || len(rep.Findings) != 0 || len(rep.Notes) != 0 {
		t.Errorf("no analyzers: %+v, %v", rep, err)
	}
}

func TestBadGOARCH(t *testing.T) {
	p := loadProgram(t, manyFindings(), ".")
	if _, err := Run(p.prog, p.res, Config{Roots: p.prog.Roots, Analyzers: Analyzers, GOARCH: "nonesuch"}); err == nil {
		t.Error("an unknown GOARCH is not an error")
	}
}

// TestShiftUsesGOARCH: int is 32 bits on 386 and 64 on amd64.
func TestShiftUsesGOARCH(t *testing.T) {
	src := map[string]string{"main.gp": "package main\n\nfunc main() {\n\tx := 1\n\t_ = x << 40\n}\n"}
	p := loadProgram(t, src)
	run := func(arch string) int {
		rep, err := Run(p.prog, p.res, Config{Roots: p.prog.Roots, Analyzers: []*Analyzer{Lookup("shift")}, GOARCH: arch})
		if err != nil {
			t.Fatal(err)
		}
		return len(rep.Findings)
	}
	if n := run("amd64"); n != 0 {
		t.Errorf("amd64: %d findings", n)
	}
	if n := run("386"); n != 1 {
		t.Errorf("386: %d findings, want 1", n)
	}
}

// TestManyPackages vets 100 packages that import each other in a chain, as
// the speed test of the step does: the facts of printf travel along the
// chain, and the findings are exact. (The time of this is reported by
// BenchmarkManyPackages; a test must not depend on the clock.)
func TestManyPackages(t *testing.T) {
	p := manyPackages(t, 100)
	rep := p.vet(t)
	if len(rep.Findings) != 1 {
		t.Fatalf("want 1 finding, got %+v", rep.Findings)
	}
	if f := rep.Findings[0]; f.Analyzer != "printf" || !strings.Contains(f.Message, "Wrap0") {
		t.Errorf("finding: %+v", f)
	}
}

func BenchmarkManyPackages(b *testing.B) {
	p := manyPackages(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.vet(b)
	}
}

func manyPackages(t testing.TB, n int) *program {
	files := map[string]string{}
	for i := 0; i < n; i++ {
		var imp, call string
		if i+1 < n {
			imp = fmt.Sprintf("import \"example.com/m/p%d\"\n\n", i+1)
			call = fmt.Sprintf("\treturn p%d.Wrap%d(format, args...)", i+1, i+1)
		} else {
			imp = "import \"fmt\"\n\n"
			call = "\treturn fmt.Sprintf(format, args...)"
		}
		files[fmt.Sprintf("p%d/p%d.gp", i, i)] = fmt.Sprintf("package p%d\n\n%sfunc Wrap%d(format string, args ...any) string {\n%s\n}\n\nfunc Other%d(x int) int {\n\tif x == 0 {\n\t\treturn 1\n\t}\n\treturn x * 2\n}\n", i, imp, i, call, i)
	}
	files["main.gp"] = "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/m/p0\"\n)\n\nfunc main() { fmt.Println(p0.Wrap0(\"%d\", \"x\")) }\n"
	return loadProgram(t, files, ".")
}
