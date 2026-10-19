package cppgen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The tests of -gp:concepts (Phase 07 step 7.9, Config.Concepts, lower/concepts.go): the concepts
// in the package headers accept the C++ types that satisfy a Go constraint and reject the others,
// the output without the option has none, and a program with the option runs as without it.

// conceptsDir returns the folder of the module of the tests.
func conceptsDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "concepts"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeProgram emits the module in dir with cfg and writes the files below root.
func writeProgram(t *testing.T, dir, root string, cfg Config) *ProgramFiles {
	t.Helper()
	fset, pkgs := loadProgram(t, dir, nil, nil)
	files, err := EmitProgram(fset, pkgs, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files.Files {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// TestConceptsSatisfy compiles check.cpp.txt, whose static_asserts say which C++ types satisfy each
// concept of package k (positive and negative), and snippets that must be rejected with an error
// that names the constraint, with each compiler.
func TestConceptsSatisfy(t *testing.T) {
	dir := conceptsDir(t)
	check, err := os.ReadFile(filepath.Join(dir, "check.cpp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "gen")
	writeProgram(t, dir, root, Config{Concepts: true})
	src := filepath.Join(root, "check.cpp")
	if err := os.WriteFile(src, check, 0o644); err != nil {
		t.Fatal(err)
	}
	// A negative test is a use of a concept that C++ must reject; the compiler's message names it.
	const head = "#include \"gpkg/example.com/concepts/k/pkg.hpp\"\nnamespace k = gpkg_example_dcom__concepts__k;\n" +
		"template <typename T> requires k::gp_sat_Sum<T> T twice(T x) { return x; }\n" +
		"template <typename T> requires k::gp_sat_Rename<T> void rename(T x) {}\n"
	negatives := []struct{ name, code, want string }{
		{"string is not a Number", "int main() { twice(gp::string(\"a\")); }", "gp_sat_Sum"},
		{"unsigned is not a Number", "int main() { twice(uint8_t(1)); }", "gp_sat_Sum"},
		{"a value has no pointer method", "int main() { rename(k::Named{}); }", "gp_sat_Rename"},
		{"static_assert", "static_assert(k::gp_sat_Len<gp::string, uint8_t>);", "gp_sat_Len"},
	}
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			if ok, out, err := c.syntaxOK(src, includeFlag(t), "-iquote", root); err != nil || !ok {
				t.Errorf("check.cpp does not compile: %v\n%s", err, out)
			}
			for i, n := range negatives {
				neg := filepath.Join(root, "neg"+string(rune('a'+i))+".cpp")
				if err := os.WriteFile(neg, []byte(head+n.code+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				ok, out, err := c.syntaxOK(neg, includeFlag(t), "-iquote", root)
				if err != nil {
					t.Fatal(err)
				}
				if ok || !strings.Contains(out, n.want) {
					t.Errorf("%s: compiled=%v, want an error that names %s:\n%s", n.name, ok, n.want, out)
				}
			}
		})
	}
}

// TestConceptsOff: without Config.Concepts the files have no concept and do not include the
// concepts header; with it they have both; and the files of a program without constraints or
// interfaces are the same either way (but for the forwarders it turns on).
func TestConceptsOff(t *testing.T) {
	dir := conceptsDir(t)
	root := filepath.Join(t.TempDir(), "gen")
	off := snapshot(emitProgram(t, dir, root), dir)
	if strings.Contains(off, "concept gp_") || strings.Contains(off, "gocpp/concepts.hpp") || strings.Contains(off, "gp_sat_") {
		t.Errorf("the output without -gp:concepts mentions concepts")
	}
	fset, pkgs := loadProgram(t, dir, nil, nil)
	files, err := EmitProgram(fset, pkgs, root, Config{Concepts: true})
	if err != nil {
		t.Fatal(err)
	}
	on := snapshot(files, dir)
	for _, w := range []string{"#include <gocpp/concepts.hpp>", "concept gp_concept_Integer =", "concept gp_sat_Len =", "::gp::concepts::tilde<S, gp::slice<E>>"} {
		if !strings.Contains(on, w) {
			t.Errorf("the output with -gp:concepts has no %q", w)
		}
	}
	// The main package has no constraint: its header is the same.
	if got, want := section(on, "gpkg/example.com/concepts/pkg.hpp"), section(off, "gpkg/example.com/concepts/pkg.hpp"); got != want {
		t.Errorf("pkg.hpp of main differs:\n%s\nwant\n%s", got, want)
	}
}

// section returns the file path of a snapshot.
func section(snap, path string) string {
	_, rest, _ := strings.Cut(snap, "//// "+path+"\n")
	text, _, _ := strings.Cut(rest, "\n//// ")
	return text
}

// TestConceptsRun: the program that uses the generic functions of k builds and prints what real
// Go prints when the concepts are written too.
func TestConceptsRun(t *testing.T) {
	const want = "6 3.5 b 3 true\nhello a hello w tagged 4 2 3\n2 2 hello both tagged 5 tagged 6\nz 1\n"
	checkProgramRuns(t, conceptsDir(t), nil, nil, want, Config{Concepts: true})
}

// concepts2Dir returns the folder of the second module: names that the text of a concept uses, a
// method named like its type, a variadic method, a defined pointer type, a Unicode name, a
// constraint of another package.
func concepts2Dir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "concepts2"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestConceptsNames compiles check.cpp.txt of the second module with each compiler: its
// static_asserts are what a C++ user can ask, also of the cases that once made pkg.hpp not compile
// (type and type parameter names x, a0, std, gp) or made a concept reject what Go accepts (a method named
// like its struct, which the struct writes with a _) or accept what Go rejects (a defined pointer type).
func TestConceptsNames(t *testing.T) {
	dir := concepts2Dir(t)
	check, err := os.ReadFile(filepath.Join(dir, "check.cpp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "gen")
	writeProgram(t, dir, root, Config{Concepts: true})
	src := filepath.Join(root, "check.cpp")
	if err := os.WriteFile(src, check, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			if ok, out, err := c.syntaxOK(src, includeFlag(t), "-iquote", root); err != nil || !ok {
				t.Errorf("check.cpp does not compile: %v\n%s", err, out)
			}
		})
	}
}

// TestConceptsNamesRun: the program of the second module builds with the concepts and prints what
// real Go prints.
func TestConceptsNamesRun(t *testing.T) {
	const want = "3 s 3 s número 5 0\n3 s número 8\n5 7\n"
	checkProgramRuns(t, concepts2Dir(t), nil, nil, want, Config{Concepts: true})
}

// TestConceptsCorpus emits the run tests of interfaces and about a twenty-fifth of the others (tests/run: a share by a hash of the
// file name) with Config.Concepts and compiles
// the C++ (syntax only) with each compiler: the concepts must not break a package, whatever names
// and types it has. A test that the emitter doesn't take yet is skipped.
func TestConceptsCorpus(t *testing.T) {
	progs, err := filepath.Glob(filepath.Join("..", "..", "tests", "run", "*.gp"))
	if err != nil || len(progs) == 0 {
		t.Fatalf("no run tests: %v", err)
	}
	pick := regexp.MustCompile(`^(iface_|generics_iface_)`)
	var cs []*cxx
	for _, c := range compilers(t) {
		if c.missing == "" {
			cs = append(cs, c)
		}
	}
	if len(cs) == 0 {
		t.Skip("no C++ compiler")
	}
	checked := 0
	for _, prog := range progs {
		base := strings.TrimSuffix(filepath.Base(prog), ".gp")
		if !pick.MatchString(base) && fnv32(base)%25 != 0 {
			continue
		}
		checked++
		t.Run(base, func(t *testing.T) {
			t.Parallel()
			abs, err := filepath.Abs(prog)
			if err != nil {
				t.Fatal(err)
			}
			cpp, err := emitPatterns(t, Config{Concepts: true}, filepath.Dir(abs), abs)
			if err != nil {
				t.Skipf("not emitted yet: %v", err)
			}
			work := t.TempDir()
			file := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(file, []byte(cpp), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range cs {
				if ok, out, err := c.syntaxOK(file, includeFlag(t)); err != nil || !ok {
					t.Errorf("%s: the C++ with concepts does not compile: %v\n%s", c.name, err, out)
				}
			}
		})
	}
	if checked < 40 {
		t.Fatalf("only %d run tests checked", checked)
	}
}
