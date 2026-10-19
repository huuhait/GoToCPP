package fuzz

import (
	"bytes"
	"errors"
	"flag"
	"go/ast"
	gofmt "go/format"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/printer"
	"github.com/huuhait/gocpp/tools/fuzz/fuzzkit"
)

// The fuzz targets of the pipeline. Every one runs its seeds (all the .gp files of tests/,
// examples/ and stdlib/, and a few programs of the generator) as an ordinary test.
//
//	go test -run '^Fuzz' ./tools/fuzz                       the seeds of all targets
//	go test -fuzz '^FuzzCheck$' -fuzztime 5m ./tools/fuzz   one target
//
// What a finding is: a panic, a hang (more than GOCPP_FUZZ_HANG seconds, 10), a live heap of more
// than GOCPP_FUZZ_MEM megabytes (3000), an "internal error" diagnostic on a program without syntax
// errors, a program that the checker accepts and the emitter refuses, output that differs between
// two runs, a wrong #line directive, and (the sampled stage, GOCPP_FUZZ_COMPILE_EVERY=N) C++ that
// g++ or clang++ refuses.

func addSeeds(f *testing.F, seeds [][]byte) {
	for _, s := range seeds {
		f.Add(s)
	}
}

// addSome adds every k-th seed in a plain "go test" run (the slow targets: a seed takes tens of
// milliseconds, a thousand seeds make a minute), and all of them when the engine is fuzzing (the
// campaign needs the whole corpus) or when GOCPP_FUZZ_ALL_SEEDS is set.
func addSome(f *testing.F, seeds [][]byte, k int) {
	if fl := flag.Lookup("test.fuzz"); (fl != nil && fl.Value.String() != "") || os.Getenv("GOCPP_FUZZ_ALL_SEEDS") != "" {
		k = 1
	}
	for i, s := range seeds {
		if i%k == 0 {
			f.Add(s)
		}
	}
}

// The syntax: the parser never panics, ends, and gives a tree with sane positions.

func FuzzParse(f *testing.F) {
	addSeeds(f, fuzzkit.GPSeeds())
	addSeeds(f, phase10Seeds())
	f.Fuzz(func(t *testing.T, src []byte) {
		fuzzkit.Guard("FuzzParse", func() {
			for _, mode := range []parser.Mode{
				parser.AllErrors | parser.ParseComments,
				parser.AllErrors | parser.ParseComments | parser.SkipObjectResolution | parser.Phase10Syntax,
				parser.DeclarationErrors,
				parser.ImportsOnly | parser.ParseComments,
				parser.PackageClauseOnly,
			} {
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "fuzz.gp", src, mode)
				if file == nil {
					continue
				}
				if err == nil && mode&(parser.ImportsOnly|parser.PackageClauseOnly) == 0 {
					checkTree(t, fset, file, len(src))
					var out bytes.Buffer
					if perr := printer.Fprint(&out, fset, file); perr != nil {
						t.Fatalf("mode %d: printing the tree of a file that parses: %v", mode, perr)
					}
					// The printer alone may drop the parentheses of a composite literal in the header of
					// a for or if (go/printer stripParens): re-parsing its output is FuzzFormat's job,
					// which has the formatter's keepParens.
				}
				gpast.Leftovers(file) // the Phase 10 recognizers read any tree
			}
		})
	})
}

// phase10Seeds are the syntax of Phase 10 (must names, macros, lazy parameters, lines of macros).
func phase10Seeds() [][]byte {
	var out [][]byte
	for _, s := range []string{
		"package p\n\nfunc f() {\n\tx := os.ReadFile!(path)\n\ty := parse![int](x)\n\tdefer f.Close!()\n\tg!()\n}\n\nfunc loadConfig!(path string) Config { return cfg }\n\nfunc (s *S) Start!() { s.ln.Close!() }\n",
		"package p\n\nmacro Assert(cond bool, msg lazy string) {\n\tif !cond {\n\t\tpanic(msg())\n\t}\n}\n\nmacro Dbg[T any](v T) T { return v }\n\nmacro VM() {\n\tvmEnter(#func())\n\tdefer vmExit(#func())\n\t#body()\n}\n",
		"package p\n\n#util.Trace()\n#Recover()\nfunc loadConfig!(path string) Config {\n\t#util.Assert(path != \"\", \"empty\")\n\treturn cfg\n}\n\n#Derive(String, Equal)\ntype Color int\n\n#Derive(JSON)\ntype (\n\tA int\n\tB int\n)\n",
		"package p\nfunc f() { F!(x); F![int](y); a.F!(z); #M(1); #a.b.c(2); x := #line() }\n",
		"package p\nmacro M(a lazy int, b lazy) {}\nfunc !() {}\nmacro () {}\n#\n#(\nF!\nF![\n",
	} {
		out = append(out, []byte(s))
	}
	return out
}

// checkTree checks that the positions of a tree that parsed without errors are inside the file
// and that a node ends after it starts.
func checkTree(t *testing.T, fset *token.FileSet, file *ast.File, size int) {
	t.Helper()
	tf := fset.File(file.Pos())
	if tf == nil {
		t.Fatal("the file is not in the file set")
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if _, ok := n.(*ast.File); ok {
			return true
		}
		pos, end := n.Pos(), n.End()
		if !pos.IsValid() {
			return true // synthesized nodes of the resolver or of the Phase 10 marks
		}
		if end < pos {
			t.Fatalf("%T at %s ends (%d) before it starts (%d)", n, fset.Position(pos), end, pos)
		}
		if int(end)-tf.Base() > size+1 || int(pos)-tf.Base() > size {
			t.Fatalf("%T at %s is outside the file (size %d)", n, fset.Position(pos), size)
		}
		return true
	})
}

// The formatter: what it makes parses, is the same program, and formatting it again changes nothing.

func FuzzFormat(f *testing.F) {
	addSeeds(f, fuzzkit.GPSeeds())
	addSeeds(f, phase10Seeds())
	f.Fuzz(func(t *testing.T, src []byte) {
		fuzzkit.Guard("FuzzFormat", func() {
			once, err := format.Source("fuzz.gp", src)
			if err != nil {
				return // syntax errors, bad directives, or a literal in a header that can't keep its parentheses
			}
			twice, err := format.Source("fuzz.gp", once)
			if err != nil {
				t.Fatalf("the formatted file does not format: %v\n%s", err, once)
			}
			// Where gofmt itself is not idempotent (build constraints: "//\n//go:build\npackage A" moves
			// the line in two steps; a statement that is only ";"), that is not gocpp's bug: the
			// oracle is go/format on the same text.
			if !bytes.Equal(emptyBodies(once), emptyBodies(twice)) && !gofmtNotIdempotent(src) {
				t.Fatalf("format is not idempotent:\n--- once\n%s\n--- twice\n%s", once, twice)
			}
			a := dumpOf(t, src)
			b := dumpOf(t, once)
			if a != b {
				t.Fatalf("format changed the program:\n--- before\n%s\n--- after\n%s\n--- formatted\n%s", a, b, once)
			}
		})
	})
}

// emptyBodies turns "{  }" into "{}": the printer writes a body that is only ";" with two spaces,
// and the next pass (the ";" is gone) with none. gofmt does that for func, and the same printer
// does it for macro.
func emptyBodies(b []byte) []byte {
	return emptyBodyRx.ReplaceAll(b, []byte("{}"))
}

var emptyBodyRx = regexp.MustCompile(`\{ +\}`)

// gofmtNotIdempotent reports whether go/format's own output for src changes when it is formatted
// again (or src is not Go that go/format reads: then gocpp's extensions are in it, and the check
// stays on).
func gofmtNotIdempotent(src []byte) bool {
	one, err := gofmt.Source(src)
	if err != nil {
		return false
	}
	two, err := gofmt.Source(one)
	return err == nil && !bytes.Equal(one, two)
}

func dumpOf(t *testing.T, src []byte) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "fuzz.gp", src, format.ParserMode&^parser.ParseComments)
	if err != nil {
		t.Fatalf("a file that formats does not parse: %v\n%s", err, src)
	}
	return fuzzkit.Dump(file)
}

// The checker: a program, however broken, gets diagnostics and no panic; a program without syntax
// errors never gets an internal error.

func FuzzCheck(f *testing.F) {
	addSome(f, fuzzkit.GPSeeds(), 4)
	addSeeds(f, genSeeds())
	f.Fuzz(func(t *testing.T, src []byte) {
		fuzzkit.Guard("FuzzCheck", func() {
			r := fuzzkit.Run(src, false)
			if bad := newInternal(r.Internal); r.Syntax && len(bad) > 0 {
				t.Fatalf("internal error on a program without syntax errors:\n%s", strings.Join(bad, "\n"))
			}
		})
	})
}

// Lowering and emitting: a program that checks cleanly lowers, without a panic and without an
// internal error.

func FuzzLower(f *testing.F) {
	addSome(f, fuzzkit.ProgramSeeds(), 6)
	addSeeds(f, genSeeds())
	f.Fuzz(func(t *testing.T, src []byte) {
		fuzzkit.Guard("FuzzLower", func() {
			r := fuzzkit.Run(src, true)
			checkEmitted(t, r)
		})
	})
}

// checkEmitted fails the test for an internal error of the check or of the emitter.
func checkEmitted(t *testing.T, r *fuzzkit.Result) {
	t.Helper()
	if bad := newInternal(r.Internal); r.Syntax && len(bad) > 0 {
		t.Fatalf("internal error on a program without syntax errors:\n%s", strings.Join(bad, "\n"))
	}
	if r.EmitErr == nil {
		return
	}
	var ce *cppgen.Error
	if errors.As(r.EmitErr, &ce) && ce.User {
		return
	}
	if msg := knownGap(r.EmitErr.Error()); msg != "" {
		t.Skipf("known gap: %s", msg)
	}
	t.Fatalf("the checker accepts the program and the emitter refuses it: %v", r.EmitErr)
}

// The whole emit stage: also the same bytes twice, valid #line directives, and the sampled C++
// compile (GOCPP_FUZZ_COMPILE_EVERY).

func FuzzEmit(f *testing.F) {
	addSome(f, fuzzkit.ProgramSeeds(), 8)
	addSeeds(f, genSeeds())
	f.Fuzz(func(t *testing.T, src []byte) {
		emitChecks(t, "FuzzEmit", src)
	})
}

func emitChecks(t *testing.T, what string, src []byte) {
	t.Helper()
	var r *fuzzkit.Result
	fuzzkit.Guard(what, func() {
		r = fuzzkit.Run(src, true)
		checkEmitted(t, r)
		if r.Files == nil {
			return
		}
		again := fuzzkit.Run(src, true)
		if again.EmitErr != nil {
			t.Fatalf("the second emit of the same program fails: %v", again.EmitErr)
		}
		if err := fuzzkit.SameFiles(r.Files, again.Files); err != nil {
			t.Fatal(err)
		}
		if err := fuzzkit.CheckLines(src, r.Files); err != nil {
			t.Fatalf("#line: %v", err)
		}
	})
	if r.Files == nil || !fuzzkit.Sampled(src) {
		return
	}
	fail, err := fuzzkit.CompileAll(r)
	if err != nil {
		t.Skipf("C++ compile stage: %v", err)
	}
	if fail != nil {
		t.Fatalf("C++ compile of an accepted program:\n%v", fail)
	}
}
