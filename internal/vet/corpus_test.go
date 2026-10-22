package vet

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// sweepFile loads one .gp file as a program and vets it with all analyzers
// (the ones that are Opaque or not, as configured). It returns false when
// the program has errors (an error test, or a feature of a later phase).
func sweepFile(t *testing.T, file string, force bool) (*Report, bool) {
	dir := filepath.Dir(file)
	prog := load.Load(load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64", AllErrors: true}, file)
	res := check.Check(prog, fakeProbeConfig())
	if prog.Errors.HasErrors() {
		return nil, false
	}
	for _, lp := range prog.Packages {
		cp := res.Package(lp)
		if cp == nil || !cp.Checked() {
			return nil, false
		}
		for _, d := range cp.Errors.Diagnostics() {
			if d.Severity == diag.Error && !d.Gate {
				return nil, false
			}
		}
	}
	as := slices.Clone(Analyzers)
	if force {
		for i, a := range as {
			c := *a
			c.Opaque = true
			as[i] = &c
		}
	}
	rep, err := Run(prog, res, Config{Roots: prog.Roots, Analyzers: as})
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return rep, true
}

func TestSweepExplore(t *testing.T) {
	if os.Getenv("VET_SWEEP") == "" {
		t.Skip()
	}
	var files []string
	for _, pat := range strings.Fields(os.Getenv("VET_SWEEP")) {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	slices.Sort(files)
	n, vetted := 0, 0
	for _, f := range files {
		rep, ok := sweepFile(t, f, os.Getenv("VET_FORCE") != "")
		n++
		if !ok {
			continue
		}
		vetted++
		for _, c := range rep.Crashed() {
			fmt.Printf("CRASH %s: %s %s\n", f, c.Analyzer, c.Text)
		}
		for _, fd := range rep.Findings {
			fmt.Printf("FINDING %s:%d:%d: [%s] %s\n", filepath.Base(fd.Pos.Filename), fd.Pos.Line, fd.Pos.Column, fd.Analyzer, fd.Message)
		}
	}
	fmt.Printf("files %d vetted %d\n", n, vetted)
}

// corpusFiles returns the .gp files of the repository that use C/C++ names
// (the interop tests, the header zoo, the examples) or are programs of
// every kind (tests/run, tests/golden).
func corpusFiles(t *testing.T, patterns ...string) []string {
	t.Helper()
	var files []string
	for _, pat := range patterns {
		m, err := filepath.Glob(filepath.Join("..", "..", filepath.FromSlash(pat)))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	slices.Sort(files)
	if len(files) == 0 {
		t.Fatalf("no files for %v", patterns)
	}
	return files
}

// knownFindings are findings of the corpus that are right: the program
// does what the analyzer says. The key is the file name and the analyzer.
var knownFindings = map[string]bool{
	"unsafe_builder.gp stdmethods": true, // WriteByte without the error result of io.ByteWriter

	// The tests of the C conversions do what the analyzers say, on purpose.
	"c_constant_precision.gp narrowing": true, // C.int(big) with an int64 and C.char(b) with a uint16: they show the cut
	"float_conversions.gp narrowing":    true, // C.float(x) with a float64: it shows the rounding
	"cgo_helpers.gp cstr":               true, // C.CString("a\x00b"): it shows that the NUL ends the C string
	"umacro_lazy.gp unusedlazy":         true, // macro Never: a lazy argument that is never evaluated, as the test shows

	// A store of &y into a variable of the enclosing function or block: the compiler boxes y when the
	// variable leaves (ADR-139), and the analyzer does not follow that (cpplocal reports the store).
	"cvar_addr_generics.gp cpplocal":         true,
	"cvar_addr_literals.gp cpplocal":         true,
	"cvar_addr_package_literals.gp cpplocal": true,
}

// TestOpaqueCorpus runs every analyzer over the programs that use C/C++
// names: nothing crashes (x/tools analyzers do on such code unless guarded:
// printf, nilness), and nothing is found that is not a real mistake.
func TestOpaqueCorpus(t *testing.T) {
	files := corpusFiles(t, "tests/interop/*.gp", "tests/zoo/*.gp", "examples/*/main.gp")
	vetted := 0
	for _, f := range files {
		rep, ok := sweepFile(t, f, false)
		if !ok {
			continue // an error test, or a feature of a later phase
		}
		vetted++
		for _, c := range rep.Crashed() {
			t.Errorf("%s: analyzer %s crashed: %s", f, c.Analyzer, strings.SplitN(c.Text, "\n", 2)[0])
		}
		for _, fd := range rep.Findings {
			if !knownFindings[filepath.Base(fd.Pos.Filename)+" "+fd.Analyzer] {
				t.Errorf("%s:%d:%d: unexpected finding: [%s] %s", filepath.Base(fd.Pos.Filename), fd.Pos.Line, fd.Pos.Column, fd.Analyzer, fd.Message)
			}
		}
	}
	if vetted < 200 {
		t.Errorf("only %d of %d files could be vetted", vetted, len(files))
	}
}

// TestRunCorpusNoCrash vets the programs of tests/run and tests/golden with
// every analyzer: none crashes. (They have findings: the tests of the
// language do what vet warns about, on purpose.)
func TestRunCorpusNoCrash(t *testing.T) {
	files := corpusFiles(t, "tests/run/*.gp", "tests/golden/*.gp")
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
	}
	if vetted < 500 {
		t.Errorf("only %d of %d files could be vetted", vetted, len(files))
	}
}
