package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// textDocument/declaration, the definitions in the sources of a package and the hover of the
// structs that typedefs hide (step 12.13e), against the real compiler: the answers wait for the
// background workers of the snapshot and never block a request.

const geoHeader = `#pragma once
// A 2D vector.
typedef struct Vector2 {
    float x;   // X component
    float y;   // Y component
} Vector2;

/** Pixel hues. */
typedef enum { RED, GREEN = 5 } Hue;

typedef void (*Callback)(int);

// Adds numbers.
int add(int a, int b);
int declaredOnly(int a);
extern int counter;
Vector2 origin(void);
`

const geoSource = `#include "geo.h"

int counter = 3;

int add(int a, int b) {
    return a + b;
}

Vector2 origin(void) {
    Vector2 zero = {0, 0};
    return zero;
}
`

const geoMain = `package main
import "./geo.h"

func helper() int { return 1 }

func main() {
	_ = geo.add(1, 2)
	_ = geo.declaredOnly(1)
	_ = geo.counter
	v := geo.origin()
	var w geo.Vector2
	var h geo.Hue
	var cb geo.Callback
	_, _, _, _ = v, w, h, cb
	_ = helper()
}
`

func startGeo(t *testing.T, init string) *dclient {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	c := startNav(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/geo\n",
		"main.gp":   geoMain,
		"geo.h":     geoHeader,
		"geo.cpp":   geoSource,
	}, init)
	t.Cleanup(func() { c.wait() })
	c.open("main.gp", geoMain)
	return c
}

// waitLocation polls the request at mark until it has a location in the file with that name.
func waitLocation(t *testing.T, c *dclient, method, mark, base string) protocol.Location {
	t.Helper()
	deadline := time.Now().Add(hangLimit)
	for {
		locs := c.locationsAt(method, "main.gp", geoMain, mark)
		for _, loc := range locs {
			if filepath.Base(loc.URI.Path()) == base {
				return loc
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s at %q never had a location in %s: %v; logs: %s", method, mark, base, locs,
				c.logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func lineOf(text, needle string) uint32 {
	return uint32(strings.Count(text[:strings.Index(text, needle)], "\n"))
}

func TestDeclarationGoesToTheHeaderAndDefinitionToTheSource(t *testing.T) {
	c := startGeo(t, plainInit)
	const declaration, definition = "textDocument/declaration", "textDocument/definition"
	cases := []struct {
		name, mark, needle string
	}{
		{"function", "_ = geo.ad|d(1, 2)", "add("},
		{"variable", "_ = geo.coun|ter", "counter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := waitLocation(t, c, declaration, tc.mark, "geo.h")
			if want := lineOf(geoHeader, tc.needle); header.Range.Start.Line != want {
				t.Errorf("declaration line = %d, want %d (the header)",
					header.Range.Start.Line, want)
			}
			body := waitLocation(t, c, definition, tc.mark, "geo.cpp")
			if want := lineOf(geoSource, tc.needle); body.Range.Start.Line != want {
				t.Errorf("definition line = %d, want %d (the source)", body.Range.Start.Line, want)
			}
		})
	}
	t.Run("a function that no source defines has the header for both", func(t *testing.T) {
		for _, method := range []string{declaration, definition} {
			loc := waitLocation(t, c, method, "_ = geo.declaredOn|ly(1)", "geo.h")
			if want := lineOf(geoHeader, "declaredOnly"); loc.Range.Start.Line != want {
				t.Errorf("%s line = %d, want %d", method, loc.Range.Start.Line, want)
			}
		}
	})
	t.Run("a type has the header for both", func(t *testing.T) {
		for _, method := range []string{declaration, definition} {
			loc := waitLocation(t, c, method, "var w geo.Vec|tor2", "geo.h")
			if want := lineOf(geoHeader, "} Vector2;"); loc.Range.Start.Line != want {
				t.Errorf("%s line = %d, want %d", method, loc.Range.Start.Line, want)
			}
		}
	})
}

func TestDeclarationOfAGoNameIsItsDefinition(t *testing.T) {
	c := startGeo(t, plainInit)
	const mark = "_ = hel|per()"
	declared := c.locationsAt("textDocument/declaration", "main.gp", geoMain, mark)
	defined := c.locationsAt("textDocument/definition", "main.gp", geoMain, mark)
	if len(declared) != 1 || len(defined) != 1 || declared[0] != defined[0] {
		t.Fatalf("declaration %v, definition %v, want the same place", declared, defined)
	}
	if want := lineOf(geoMain, "func helper"); declared[0].Range.Start.Line != want {
		t.Errorf("line = %d, want %d", declared[0].Range.Start.Line, want)
	}
}

func TestDeclarationWithoutFactsIsEmptyAndDoesNotAsk(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "opaque")
	c := startNav(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/geo\n",
		"main.gp":   geoMain,
		"geo.h":     geoHeader,
		"geo.cpp":   geoSource,
	}, plainInit)
	t.Cleanup(func() { c.wait() })
	c.open("main.gp", geoMain)
	const mark = "_ = geo.ad|d(1, 2)"
	if got := c.locationsAt("textDocument/declaration", "main.gp", geoMain, mark); len(got) != 0 {
		t.Errorf("declaration without facts = %v, want none", got)
	}
}

func TestHoverOfATypedefShowsTheStructWithItsFields(t *testing.T) {
	c := startGeo(t, plainInit)
	text := waitHover(t, c, geoMain, "var w geo.Vec|tor2",
		"float y; // Y component")
	for _, want := range []string{
		"typedef struct Vector2 {\n    float x; // X component\n" +
			"    float y; // Y component\n} Vector2;",
		"A 2D vector.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("hover lacks %q: %q", want, text)
		}
	}
	if strings.Contains(text, "typedef struct Vector2 Vector2;") {
		t.Errorf("the hover still shows the typedef without its struct: %q", text)
	}
}

func TestHoverOfAnEnumTypedefShowsItsEnumerators(t *testing.T) {
	c := startGeo(t, markdownInit)
	text := waitHover(t, c, geoMain, "var h geo.H|ue", "GREEN = 5")
	if !strings.Contains(text, "```cpp\ntypedef enum {\n    RED,\n    GREEN = 5,\n} Hue;\n```") ||
		!strings.Contains(text, "Pixel hues.") {
		t.Errorf("hover = %q", text)
	}
}

func TestHoverOfAFunctionPointerTypedefStaysTheTypedef(t *testing.T) {
	c := startGeo(t, plainInit)
	text := waitHover(t, c, geoMain, "var cb geo.Call|back", "typedef void (*Callback)(int);")
	if strings.Contains(text, "{") {
		t.Errorf("hover of a function pointer typedef has a body: %q", text)
	}
}

func TestHoverOfAVariableShowsItsTypeWithTheFields(t *testing.T) {
	c := startGeo(t, markdownInit)
	text := waitHover(t, c, geoMain, "\tvar w| geo.Vector2", "float y; // Y component")
	for _, want := range []string{
		"```cpp\nVector2\n```",
		"C/C++ native type of variable `w`.",
		"A 2D vector.",
		"```cpp\ntypedef struct Vector2 {\n    float x; // X component\n" +
			"    float y; // Y component\n} Vector2;\n```",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("hover lacks %q: %q", want, text)
		}
	}
	plain := startGeo(t, plainInit)
	got := waitHover(t, plain, geoMain, "\tvar w| geo.Vector2", "float y; // Y component")
	if strings.Contains(got, "```") {
		t.Errorf("a plain hover has a fence: %q", got)
	}
}

func TestHoverOfAVariableOfAFunctionResultShowsItsTypeToo(t *testing.T) {
	c := startGeo(t, plainInit)
	waitHover(t, c, geoMain, "\tv| := geo.origin()", "float x; // X component")
}

func TestPairDefinitions(t *testing.T) {
	header := func(line int, typ, params string) probe.Declaration {
		return probe.Declaration{Kind: "function", File: "/inc/api.h", Line: line, Type: typ,
			Params: params}
	}
	source := func(line int, typ, params string) probe.Declaration {
		return probe.Declaration{Kind: "function", File: "/src/api.cpp", Line: line, Type: typ,
			Params: params, Defined: true, Redeclares: true}
	}
	t.Run("overloads are paired by their parameters, not by their order", func(t *testing.T) {
		decls := []probe.Declaration{
			header(1, "int (int)", "int"), header(2, "int (double)", "double")}
		defs := []probe.Declaration{source(20, "int (double)", "double")}
		got := pairDefinitions(decls, defs)
		if got[0] != decls[0] || got[1] != defs[0] {
			t.Errorf("got %+v, want the int overload in the header, the double one in the source",
				got)
		}
	})
	t.Run("typedef names do not tell one function from another", func(t *testing.T) {
		decls := []probe.Declaration{header(1, "size_t (size_t)", "unsigned long")}
		defs := []probe.Declaration{source(3, "unsigned long (unsigned long)", "unsigned long")}
		if got := pairDefinitions(decls, defs); got[0] != defs[0] {
			t.Errorf("got %+v, want the definition", got)
		}
	})
	t.Run("a lone definition of other parameters is another function", func(t *testing.T) {
		decls := []probe.Declaration{header(1, "int (int)", "int")}
		for _, params := range []string{"char", "double", "int;int", ""} {
			defs := []probe.Declaration{source(3, "int (char)", params)}
			if got := pairDefinitions(decls, defs); got[0] != decls[0] {
				t.Errorf("parameters %q: got %+v, want the declaration", params, got)
			}
		}
	})
	t.Run("a variable is paired whatever its type says", func(t *testing.T) {
		decls := []probe.Declaration{{Kind: "variable", File: "/inc/api.h", Line: 1, Type: "int[]"}}
		defs := []probe.Declaration{{Kind: "variable", File: "/src/api.cpp", Line: 4,
			Type: "int[3]", Defined: true}}
		if got := pairDefinitions(decls, defs); got[0] != defs[0] {
			t.Errorf("got %+v, want the definition (a name has one definition)", got)
		}
	})
	t.Run("two declarations of other types are not guessed", func(t *testing.T) {
		decls := []probe.Declaration{
			header(1, "int (int)", "int"), header(2, "int (double)", "double")}
		defs := []probe.Declaration{source(3, "int (char)", "char")}
		if got := pairDefinitions(decls, defs); got[0] != decls[0] || got[1] != decls[1] {
			t.Errorf("got %+v, want both declarations", got)
		}
	})
	t.Run("a declaration that is a definition stays", func(t *testing.T) {
		inline := header(1, "int (int)", "int")
		inline.Defined = true
		defs := []probe.Declaration{source(3, "int (int)", "int")}
		if got := pairDefinitions([]probe.Declaration{inline}, defs); got[0] != inline {
			t.Errorf("got %+v, want the inline definition of the header", got)
		}
	})
	t.Run("a type is not replaced", func(t *testing.T) {
		typ := probe.Declaration{Kind: "type", File: "/inc/api.h", Line: 1}
		got := pairDefinitions([]probe.Declaration{typ}, []probe.Declaration{source(3, "", "")})
		if got[0] != typ {
			t.Errorf("got %+v", got)
		}
	})
}

func TestAddTypeBodyShowsOnlyABody(t *testing.T) {
	withBody := probe.Declaration{Kind: "type", Doc: "A vector.",
		Signature: "typedef struct V {\n    float x;\n} V;"}
	h := &hoverResult{signature: "V", text: "C/C++ native type of variable `v`."}
	addTypeBody(h, []probe.Declaration{withBody})
	if h.typeBody != withBody.Signature || !strings.Contains(h.text, "A vector.") {
		t.Errorf("hover = %+v, want the body and the comment", h)
	}
	for name, decls := range map[string][]probe.Declaration{
		"only declared":  {{Kind: "type", Signature: "typedef struct F F;"}},
		"a function":     {{Kind: "function", Signature: "int f() {"}},
		"two":            {withBody, withBody},
		"no declaration": nil,
	} {
		h := &hoverResult{signature: "V"}
		addTypeBody(h, decls)
		if h.typeBody != "" || h.text != "" {
			t.Errorf("%s: hover = %+v, want no body", name, h)
		}
	}
}

// startProject starts a server on a folder with these files and opens main.gp.
func startProject(t *testing.T, init, main string, files map[string]string) *dclient {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	files["gocpp.mod"] = "module example.com/geo\n"
	files["main.gp"] = main
	c := startNav(t, testConfig(), files, init)
	t.Cleanup(func() { c.wait() })
	c.open("main.gp", main)
	return c
}

// waitLine polls the request at mark until its location in the file base is at the line.
func waitLine(t *testing.T, c *dclient, main, method, mark, base string, line uint32) {
	t.Helper()
	deadline := time.Now().Add(hangLimit)
	var last []protocol.Location
	for {
		last = c.locationsAt(method, "main.gp", main, mark)
		for _, loc := range last {
			if filepath.Base(loc.URI.Path()) == base && loc.Range.Start.Line == line {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s at %q never was at %s:%d: %v; logs: %s", method, mark, base, line, last,
				c.logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (c *dclient) changedOnDisk(name string) {
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`,
		protocol.URIFromPath(filepath.Join(c.root, name))))
}

func TestDefinitionFollowsAnEditOfTheSourceOnDisk(t *testing.T) {
	c := startGeo(t, plainInit)
	const mark = "_ = geo.ad|d(1, 2)"
	waitLine(t, c, geoMain, "textDocument/definition", mark, "geo.cpp", lineOf(geoSource, "add("))
	edited := "// one\n// two\n// three\n" + geoSource
	if err := os.WriteFile(filepath.Join(c.root, "geo.cpp"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, release := c.s.session.Snapshot()
	before := snap.ID()
	release()
	c.changedOnDisk("geo.cpp")
	waitLine(t, c, geoMain, "textDocument/definition", mark, "geo.cpp", lineOf(edited, "add("))
	if got := lineOf(edited, "add(") - lineOf(geoSource, "add("); got != 3 {
		t.Fatalf("the edit moved the definition by %d lines, want 3", got)
	}
	snap, release = c.s.session.Snapshot()
	after := snap.ID()
	release()
	if after != before {
		t.Errorf("a source on disk made snapshot %d from %d: it is read again in the background",
			after, before)
	}
}

const linkageMain = `package main
import "./geo.h"

func main() {
	_ = geo.ovr(1)
	_ = geo.lone(1)
	_ = geo.real(1)
	_ = geo.sp(1)
	_ = geo.sp2(1)
	_ = geo.g2(1)
}
`

func TestDefinitionIgnoresAHelperOfTheSameName(t *testing.T) {
	header := "#pragma once\nint ovr(int);\nint lone(int);\nint real(int);\nint sp(int);\n" +
		"int sp2(int);\nint g2(int);\n"
	source := "#include \"geo.h\"\nstatic int ovr(char c) { return c; }\n" +
		"namespace { int lone(double d) { return 1; } }\nint real(int x) { return x; }\n" +
		"static int sp(char c);\nstatic int sp(char c) { return c; }\n" +
		"namespace { int sp2(double); int sp2(double d) { return 1; } }\n" +
		"int g2(int a) { return a; }\nint g2(int a);\n"
	c := startProject(t, plainInit, linkageMain,
		map[string]string{"geo.h": header, "geo.cpp": source})
	waitLine(t, c, linkageMain, "textDocument/definition", "_ = geo.re|al(1)", "geo.cpp", 3)
	for _, tc := range []struct{ mark, name string }{
		{"_ = geo.ov|r(1)", "ovr"}, {"_ = geo.lo|ne(1)", "lone"},
		// declared first and then defined by the source: the function is the source's own
		{"_ = geo.s|p(1)", "sp"}, {"_ = geo.sp|2(1)", "sp2"},
	} {
		locs := c.locationsAt("textDocument/definition", "main.gp", linkageMain, tc.mark)
		if len(locs) != 1 || filepath.Base(locs[0].URI.Path()) != "geo.h" ||
			locs[0].Range.Start.Line != lineOf(header, tc.name+"(") {
			t.Errorf("definition of %s = %v, want its declaration in geo.h", tc.name, locs)
		}
	}
	// a definition that a redeclaration follows in the source is still the definition
	waitLine(t, c, linkageMain, "textDocument/definition", "_ = geo.g|2(1)", "geo.cpp",
		lineOf(source, "int g2(int a) {"))
}

func TestHoverSaysThatSourcesWereNotRead(t *testing.T) {
	big := "#include \"geo.h\"\n" + strings.Repeat("//\n", 200<<10)
	c := startGeo(t, plainInit)
	// (the first project has no large source: no note)
	text := waitHover(t, c, geoMain, "_ = geo.ad|d(1, 2)", "Adds numbers.")
	if strings.Contains(text, "were not read") {
		t.Errorf("hover has a note without a source left out: %q", text)
	}
	files := map[string]string{"geo.h": geoHeader, "geo.cpp": geoSource, "big.cpp": big}
	c = startProject(t, plainInit, geoMain, files)
	text = waitHover(t, c, geoMain, "_ = geo.declaredOn|ly(1)", "may show this header")
	if !strings.Contains(text, "1 C/C++ source files") || !strings.Contains(text, "limit is 16") {
		t.Errorf("hover = %q, want the count and the limit", text)
	}
	// add is defined in a source that was read: the note would only mislead.
	waitLine(t, c, geoMain, "textDocument/definition", "_ = geo.ad|d(1, 2)", "geo.cpp",
		lineOf(geoSource, "add("))
	text = waitHover(t, c, geoMain, "_ = geo.ad|d(1, 2)", "Adds numbers.")
	if strings.Contains(text, "were not read") {
		t.Errorf("hover of a defined function has the note: %q", text)
	}
	typeHover := waitHover(t, c, geoMain, "var w geo.Vec|tor2", "Vector2")
	if strings.Contains(typeHover, "were not read") {
		t.Error("the note is on a type, which no source defines")
	}
}
