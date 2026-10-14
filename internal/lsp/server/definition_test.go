package server

import (
	"context"
	"encoding/json"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// definitions returns the locations of a definition request (method) at
// the | of mark in the file name with the text src, each as
// "file:line:col-line:col" (the file relative to the root, 1-based lines
// and UTF-16 columns), and its text.
func (c *dclient) definitions(method, name, src, mark string) []string {
	c.t.Helper()
	resp := c.call(method, c.posParams(name, src, mark))
	if resp.Error != nil {
		c.t.Fatalf("%s %q: %v", method, mark, resp.Error)
	}
	var locs []protocol.Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil || locs == nil {
		c.t.Fatalf("%s %q: %s, want a list", method, mark, resp.Result)
	}
	var list []string
	for _, l := range locs {
		path := l.URI.Path()
		rel, err := filepath.Rel(c.root, path)
		if err != nil {
			rel = path
		}
		s := fmt.Sprintf("%s:%d:%d-%d:%d", filepath.ToSlash(rel), l.Range.Start.Line+1, l.Range.Start.Character+1, l.Range.End.Line+1, l.Range.End.Character+1)
		if text, err := c.textOf(path); err == nil {
			m := protocol.NewMapper(text)
			if start, end, err := m.RangeOffsets(l.Range); err == nil && end > start {
				s += " " + text[start:end]
			}
		}
		list = append(list, s)
	}
	return list
}

// textOf returns the text of the file path: the open document's, or else
// the file's on disk.
func (c *dclient) textOf(path string) (string, error) {
	if o := c.overlay(protocol.URIFromPath(path)); o != nil {
		return o.Text(), nil
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

// A defCase is a definition request at the | of mark in app/main.gp and
// its locations (definitions).
type defCase struct {
	method, mark string
	want         []string
}

func TestDefinition(t *testing.T) {
	c := startNav(t, testConfig(), navFiles, plainInit)
	src := navFiles["app/main.gp"]
	c.open("app/main.gp", src)
	const def, typ = "textDocument/definition", "textDocument/typeDefinition"
	for _, tc := range []defCase{
		// In the file: a function, a method, a field, a type, a local.
		{def, "Jo|in(\",", []string{"app/main.gp:38:6-38:10 Join"}},
		{def, "p.Di|st(", []string{"app/main.gp:33:16-33:20 Dist"}},
		{def, "n.|X,", []string{"app/main.gp:22:2-22:3 X"}},
		{def, "Na|med{Point", []string{"app/main.gp:27:6-27:11 Named"}},
		{def, "Count += |d", []string{"app/main.gp:52:2-52:3 d"}},
		// A package name goes to its import.
		{def, "ge|om.Area", []string{"app/main.gp:8:2-8:24 \"example.com/app/geom\""}},
		// In another file of the package.
		{def, "hel|per()", []string{"app/util.gp:4:6-4:12 helper"}},
		{def, "return |Answer", nil}, // not in main.gp: see below
		// In another package (from export data): a function, a type, a
		// field.
		{def, "geom.Ar|ea", []string{"geom/geom.gp:10:6-10:10 Area"}},
		{def, "geom.Re|ct{", []string{"geom/geom.gp:5:6-5:10 Rect"}},
		{def, "Rect{W|: 2", []string{"geom/geom.gp:6:2-6:3 W"}},
		// An import path: the package clauses of the package's files.
		{def, "\"example.com/app/ge|om\"", []string{"geom/geom.gp:2:9-2:13 geom"}},
		// A header import: the header file.
		{def, "\"./engine|.hpp\"", []string{"app/engine.hpp:1:1-1:1"}},
		// Nothing for now: C/C++ names (Phase 12), the standard library
		// (embedded, no file), predeclared names, keywords.
		{def, "engine.st|art", []string{}},
		{def, "eng|ine.start", []string{}},
		{def, "fmt.Print|ln", []string{}},
		{def, "\"str|ings\"", []string{}},
		{def, "s = app|end", []string{}},
		{def, "var err er|ror", []string{}},
		{def, "fu|nc main", []string{}},
		// The type of a variable, of a function's results, of a field.
		{typ, "p := Po|int{", []string{"app/main.gp:21:6-21:11 Point"}},
		{typ, "|p := Point{", []string{"app/main.gp:21:6-21:11 Point"}},
		{typ, "n := Named{Po|int: p}", []string{"app/main.gp:21:6-21:11 Point"}},
		{typ, "geom.Area(geom.Rect{W: 2, H: 3}), hel|per()", []string{}}, // int
		{typ, "var |e *", []string{}},                                    // C/C++
		{typ, "Count += d\n\tengine.start(|e)", []string{}},
	} {
		if tc.want == nil {
			continue
		}
		got := c.definitions(tc.method, "app/main.gp", src, tc.mark)
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s at %q:\n%s\nwant\n%s", tc.method, tc.mark, strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
		}
	}
	// From the other file of the package, to main.gp.
	util := navFiles["app/util.gp"]
	if got := c.definitions(def, "app/util.gp", util, "return |Answer"); strings.Join(got, "\n") != "app/main.gp:13:7-13:13 Answer" {
		t.Errorf("definition from util.gp: %q", got)
	}
}

// TestDefinitionOldPositions: an edit of an import that keeps its API
// doesn't check its importers again (the API hash), so the positions of its
// objects in their types.Info are old; the definition still finds the
// declaration in the file as it is now.
func TestDefinitionOldPositions(t *testing.T) {
	files := noHeaderFiles()
	c := startNav(t, testConfig(), files, plainInit)
	src := files["app/main.gp"]
	c.open("app/main.gp", src)
	if got := c.definitions("textDocument/definition", "app/main.gp", src, "geom.Ar|ea"); strings.Join(got, "") != "geom/geom.gp:10:6-10:10 Area" {
		t.Fatalf("before the edit: %q", got)
	}
	_, _, checks := c.s.pipeline.Stats()
	geom := strings.Replace(navFiles["geom/geom.gp"], "package geom\n", "package geom\n\n// More lines,\n// above everything.\n", 1)
	c.open("geom/geom.gp", geom)
	got := c.definitions("textDocument/definition", "app/main.gp", src, "geom.Ar|ea")
	if strings.Join(got, "") != "geom/geom.gp:13:6-13:10 Area" {
		t.Errorf("after the edit: %q, want line 13", got)
	}
	got = c.definitions("textDocument/definition", "app/main.gp", src, "Rect{W|: 2")
	if strings.Join(got, "") != "geom/geom.gp:9:2-9:3 W" {
		t.Errorf("the field after the edit: %q, want line 9", got)
	}
	if _, _, n := c.s.pipeline.Stats(); n != checks+1 {
		t.Errorf("%d checks after the edit, want 1 (geom only: its API is the same)", n-checks)
	}
	// The position in main's types.Info is the old one indeed.
	snap, release := c.s.session.Snapshot()
	defer release()
	pkg, err := snap.PackageOf(context.Background(), c.uri("app/main.gp"))
	if err != nil {
		t.Fatal(err)
	}
	var area types.Object
	for _, imp := range pkg.Check.Types.Imports() {
		if imp.Path() == "example.com/app/geom" {
			area = imp.Scope().Lookup("Area")
		}
	}
	if area == nil {
		t.Fatal("main doesn't import geom.Area")
	}
	if line := pkg.Fset().Position(area.Pos()).Line; line != 10 {
		t.Errorf("geom.Area is at line %d in main's types, want the old line 10", line)
	}

	// With the old lines, the name nearest to the old line is of the same
	// kind as the object: here each name of lib has, a few lines before
	// it, a name spelled the same of another kind, which is nearer once
	// lines are added above.
	files = map[string]string{"gocpp.mod": "module example.com/m\n", "lib/lib.gp": oldPosLib, "app/main.gp": oldPosApp}
	c = startNav(t, testConfig(), files, plainInit)
	c.open("app/main.gp", oldPosApp)
	lib := oldPosLib
	check := func(when string) {
		t.Helper()
		for _, tc := range []struct{ mark, decl, name string }{
			{"b.|M()", "func (b B) |M", "M"},               // not the field A.M
			{"lib.|N()", "func |N()", "N"},                 // not the field CC.N
			{"lib.|Run()", "func |Run()", "Run"},           // not the method I.Run
			{"i.|Run()", "interface{ |Run() }", "Run"},     // not the function before it
			{"g.|Go()", "type G interface{ |Go() }", "Go"}, // not the field H.Go
			{"lib.|V +", "var |V = 1", "V"},                // not a parameter
			{"lib.|K,", "const |K = 2", "K"},               // not a := variable
			{"lib.|T(", "type |T int", "T"},                // not the field S.T
			{"a.|M,", "struct{ |M int }", "M"},             // not the method B.M
			{"c.|N}", "struct{ |N int }", "N"},             // not the function N
		} {
			p := at(t, lib, tc.decl)
			want := fmt.Sprintf("lib/lib.gp:%d:%d-%d:%d %s", p.Line+1, p.Character+1, p.Line+1, p.Character+1+uint32(len(tc.name)), tc.name)
			if got := c.definitions("textDocument/definition", "app/main.gp", oldPosApp, tc.mark); strings.Join(got, "") != want {
				t.Errorf("%s: at %q: %q, want %q", when, tc.mark, got, want)
			}
		}
		if text, _ := c.hover("app/main.gp", oldPosApp, "lib.|Run()"); text != "func lib.Run()\nRun is a function." {
			t.Errorf("%s: the hover of lib.Run:\n%s", when, text)
		}
	}
	check("before the edit")
	lib = strings.Replace(oldPosLib, "package lib\n", "package lib\n"+strings.Repeat("\n", 9), 1)
	c.open("lib/lib.gp", lib)
	check("after the edit")
}

// oldPosLib and oldPosApp are the package and its user of the second part
// of TestDefinitionOldPositions.
const oldPosLib = `package lib

type A struct{ M int }

type B struct{}

// M is B's method.
func (b B) M() {}

type CC struct{ N int }

// N is a function.
func N() {}

// I runs.
type I interface{ Run() }

// Run is a function.
func Run() {}

type H struct{ Go int }

func Go() {}

type G interface{ Go() }

func f(V int) int { K := V; return K }

// V is a variable.
var V = 1

// K is a constant.
const K = 2

type S struct{ T int }

// T is a type.
type T int
`

const oldPosApp = `package main

import "example.com/m/lib"

func main() {
	var b lib.B
	b.M()
	lib.N()
	lib.Run()
	var i lib.I
	i.Run()
	var g lib.G
	g.Go()
	_, _ = lib.V + lib.K, lib.T(0)
	var a lib.A
	var c lib.CC
	_, _ = a.M, c.N}
`

// TestDefinitionSystemHeader: a header in <> is looked for in the system
// folders of the C++ compiler (here a fake compiler, whose folder the test
// makes).
func TestDefinitionSystemHeader(t *testing.T) {
	sys := t.TempDir()
	writeFile(t, sys, "mylib/vec.h", "struct vec {};\n")
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) { return build.CXX{Path: "/fake/c++", Family: build.GCC}, nil }
	cfg.Probe = func(_ context.Context, _ build.CXX, _ string, _, cands []string) (interop.Result, error) {
		return interop.Result{Kinds: make(map[string]interop.Kind)}, nil
	}
	cfg.IncludeDirs = func(context.Context, build.CXX) ([]string, error) { return []string{t.TempDir(), sys}, nil }
	files := map[string]string{
		"gocpp.mod": "module example.com/h\n",
		"main.gp":   "package main\n\nimport (\n\t\"<mylib/vec.h>\"\n\t\"<nosuch.h>\"\n\tlo \"./local.h\"\n)\n\nfunc main() {}\n",
		"local.h":   "",
	}
	c := startNav(t, cfg, files, plainInit)
	src := files["main.gp"]
	c.open("main.gp", src)
	// The compiler is found in the background: wait for it.
	for i := 0; ; i++ {
		got := c.definitions("textDocument/definition", "main.gp", src, "\"<my|lib")
		if len(got) == 1 {
			if want := filepath.ToSlash(mustRel(t, c.root, filepath.Join(sys, "mylib", "vec.h"))) + ":1:1-1:1"; got[0] != want {
				t.Errorf("<mylib/vec.h>: %q, want %q", got[0], want)
			}
			break
		}
		if i == 100 {
			t.Fatal("<mylib/vec.h> is not found")
		}
		c.hover("main.gp", src, "func ma|in") // a moment
	}
	if got := c.definitions("textDocument/definition", "main.gp", src, "\"<no|such"); len(got) != 0 {
		t.Errorf("<nosuch.h>: %q", got)
	}
	for _, mark := range []string{"\"./lo|cal", "l|o \"./"} {
		if got := c.definitions("textDocument/definition", "main.gp", src, mark); strings.Join(got, "") != "local.h:1:1-1:1" {
			t.Errorf("./local.h at %q: %q", mark, got)
		}
	}
	if text, rng := c.hover("main.gp", src, "l|o \"./"); rng != "lo" || !strings.HasPrefix(text, "import lo \"./local.h\" // C/C++ header\nIts C-level names are `lo.X`") {
		t.Errorf("the hover of lo (range %q):\n%s", rng, text)
	}
	if text, _ := c.hover("main.gp", src, "\"<my|lib"); !strings.HasSuffix(text, "File: `"+filepath.Join(sys, "mylib", "vec.h")+"`") {
		t.Errorf("the hover of <mylib/vec.h>:\n%s", text)
	}
}

func mustRel(t *testing.T, base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}
