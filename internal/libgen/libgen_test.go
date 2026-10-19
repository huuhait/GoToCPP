package libgen

import (
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// The tests of the text that libgen writes. The type rules of Collect (it needs checked packages)
// are tested through the build in cmd/gocpp/modes_test.go (TestModesExportErrors).

func sampleExports() []*Export {
	pkg := &check.Package{Load: &load.Package{ImportPath: "example.com/x"}}
	exports := []*Export{
		{Name: "Add", Params: []Field{{Name: "a", C: "GoInt32"}, {Name: "b", C: "GoInt32"}}, Results: []Field{{C: "GoInt32"}},
			Pos: token.Position{Filename: "lib.gp", Line: 7}, Doc: "Add adds."},
		{Name: "Pair", Params: []Field{{C: "GoString"}, {Name: "int", C: "GoSlice"}}, Results: []Field{{C: "GoInt"}, {Name: "ok", C: "GoUint8"}},
			Pos: token.Position{Filename: "lib.gp", Line: 12}},
		{Name: "Nothing", Pos: token.Position{Filename: "lib.gp", Line: 20}},
	}
	for _, e := range exports {
		e.Pkg = pkg
	}
	return exports
}

func TestCName(t *testing.T) {
	bad := []string{"", "_", "1a", "a-b", "auto", "main", "GoInt", "_GoX", "gp_x", "gpkg_x", "gocpp_x", "GOCPP_X", "write", "malloc", "printf",
		"__x", "_Abc", "F_return", "pthread_create", "environ"}
	for _, n := range bad {
		if cName(n) == "" {
			t.Errorf("cName(%q) accepted", n)
		}
	}
	for _, n := range []string{"Add", "add", "Add_2", "_x", "writeAll", "Upper"} {
		if why := cName(n); why != "" {
			t.Errorf("cName(%q) = %q", n, why)
		}
	}
}

func TestCommentSafe(t *testing.T) {
	cases := map[string]string{
		"a */ b":     "a * / b",
		"src/*/x":    "src/ * /x",
		"a/**b":      "a/ **b",
		"back \\":    "back \\ ",
		"tri ??/":    "tri ? ?/",
		"tri ???/":   "tri ? ? ?/",
		"tri ????/":  "tri ? ? ? ?/",
		"plain text": "plain text",
	}
	for in, want := range cases {
		got := commentSafe(in)
		if got != want {
			t.Errorf("commentSafe(%q) = %q, want %q", in, got, want)
		}
		for _, bad := range []string{"*/", "/*", "??"} {
			if strings.Contains(got, bad) {
				t.Errorf("commentSafe(%q) = %q still has %q", in, got, bad)
			}
		}
		if strings.HasSuffix(got, "\\") {
			t.Errorf("commentSafe(%q) = %q ends with a backslash", in, got)
		}
	}
	if got := docText([]string{"// first */", "//second ???/"}); got != "first * /\nsecond ? ? ?/" {
		t.Errorf("docText = %q", got)
	}
}

func TestHeaderText(t *testing.T) {
	h := string(Header(HeaderInfo{Name: "libx", Mode: "c-archive", Link: []string{"-lstdc++", "-pthread"}, Exports: sampleExports()}))
	for _, want := range []string{
		// cgo's typedefs, one by one
		"typedef signed char GoInt8;", "typedef unsigned char GoUint8;", "typedef short GoInt16;", "typedef unsigned short GoUint16;",
		"typedef int GoInt32;", "typedef unsigned int GoUint32;", "typedef long long GoInt64;", "typedef unsigned long long GoUint64;",
		"typedef GoInt64 GoInt;", "typedef GoUint64 GoUint;", "typedef size_t GoUintptr;", "typedef float GoFloat32;", "typedef double GoFloat64;",
		"typedef struct { const char *p; ptrdiff_t n; } _GoString_;", "typedef _GoString_ GoString;",
		"typedef struct { void *data; GoInt len; GoInt cap; } GoSlice;", "typedef void *GoMap;", "typedef void *GoChan;",
		"typedef struct { void *t; void *v; } GoInterface;",
		"#ifndef GOCPP_LIBX_H", "Link a program with: -lstdc++ -pthread */", "#ifdef __cplusplus\nextern \"C\" {",
		"extern GOCPP_EXPORT GoInt32 Add(GoInt32 a, GoInt32 b);",
		"struct Pair_return {\n\tGoInt r0;\n\tGoUint8 r1;\n};",
		// the parameter named like a C word is renamed
		"extern GOCPP_EXPORT struct Pair_return Pair(GoString p0, GoSlice p1);",
		"extern GOCPP_EXPORT void Nothing(void);",
		"/* Add adds.\n   (example.com/x.Add) */",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("the header does not have %q:\n%s", want, h)
		}
	}
}

// The sizes of the header's types are cgo's (a C compiler checks them with _Static_assert).
func TestHeaderTypeSizes(t *testing.T) {
	cc := ""
	for _, n := range []string{"cc", "gcc", "clang"} {
		if p, err := exec.LookPath(n); err == nil {
			cc = p
			break
		}
	}
	if cc == "" {
		t.Skip("no C compiler")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.h"), Header(HeaderInfo{Name: "x", Mode: "c-archive", Exports: sampleExports()}), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `#include "x.h"
_Static_assert(sizeof(GoInt8) == 1 && sizeof(GoUint8) == 1, "8");
_Static_assert(sizeof(GoInt16) == 2 && sizeof(GoUint16) == 2, "16");
_Static_assert(sizeof(GoInt32) == 4 && sizeof(GoUint32) == 4, "32");
_Static_assert(sizeof(GoInt64) == 8 && sizeof(GoUint64) == 8 && sizeof(GoInt) == 8 && sizeof(GoUint) == 8, "64");
_Static_assert(sizeof(GoUintptr) == sizeof(void*), "uintptr");
_Static_assert(sizeof(GoFloat32) == 4 && sizeof(GoFloat64) == 8, "floats");
_Static_assert(sizeof(GoString) == 2 * sizeof(void*), "string");
_Static_assert(sizeof(GoSlice) == 3 * sizeof(void*), "slice");
_Static_assert(sizeof(GoMap) == sizeof(void*) && sizeof(GoChan) == sizeof(void*), "map chan");
_Static_assert(sizeof(GoInterface) == 2 * sizeof(void*), "interface");
_Static_assert((GoInt8)-1 < 0 && (GoUint8)-1 > 0 && (GoInt16)-1 < 0 && (GoUint16)-1 > 0 && (GoInt32)-1 < 0 && (GoUint32)-1 > 0, "signs");
_Static_assert((GoFloat32)1 / 3 != (GoFloat64)1 / 3, "float widths");
int main(void) { return 0; }
`
	if err := os.WriteFile(filepath.Join(dir, "x.c"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "-std=c11", "-Wall", "-Wextra", "-Werror", "-Wcomment", "-Wtrigraphs", "-Wno-unused-command-line-argument", "-fsyntax-only", "-I", dir, filepath.Join(dir, "x.c"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestGlueText(t *testing.T) {
	g := string(Glue(GlueInfo{
		Name: "libx", Header: "gpkg_export/libx.h", Exports: sampleExports(),
		Pkg: func(*check.Package) PkgInfo {
			return PkgInfo{Header: "gpkg/example.com/x/pkg.hpp", Namespace: "gpkg_example_dcom__x", Funcs: map[string]string{"Add": "Add", "Pair": "Pair", "Nothing": "Nothing"}}
		},
	}))
	for _, want := range []string{
		"#include \"gpkg/example.com/x/pkg.hpp\"", "#include <gocpp/lib.hpp>", "#include \"gpkg_export/libx.h\"",
		"void gp_init_all();",
		"#line 7 \"lib.gp\"", "extern \"C\" GOCPP_EXPORT GoInt32 Add(GoInt32 a, GoInt32 b) {",
		"return gp::lib::guard(&::gp_init_all, [&]() -> GoInt32 {",
		"gp::lib::in<&gpkg_example_dcom__x::Add, 0>(a)", "return gp::lib::out<GoInt32>(gpkg_example_dcom__x::Add(",
		"extern \"C\" GOCPP_EXPORT struct Pair_return Pair(GoString p0, GoSlice p1) {",
		"return Pair_return{gp::lib::out<GoInt>(std::get<0>(r)), gp::lib::out<GoUint8>(std::get<1>(r))};",
		"extern \"C\" GOCPP_EXPORT void Nothing(void) {", "gpkg_example_dcom__x::Nothing();",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("the glue does not have %q:\n%s", want, g)
		}
	}
	if n := strings.Count(g, "gp::lib::guard("); n != 3 {
		t.Errorf("%d guards for 3 exports", n)
	}
	// Every exported function adopts the calling thread and starts the runtime through guard.
	if strings.Count(g, "gp_init_all") < 4 {
		t.Errorf("the guard does not start the program:\n%s", g)
	}
}

func TestVersionScript(t *testing.T) {
	got := string(VersionScript(sampleExports()))
	want := "{\n  global:\n    Add;\n    Pair;\n    Nothing;\n  local:\n    *;\n};\n"
	if got != want {
		t.Errorf("version script:\n%s\nwant\n%s", got, want)
	}
}

func TestInitUnit(t *testing.T) {
	s := string(InitUnit("geo-lib"))
	for _, want := range []string{"namespace gocpp_geo_lib {", "void init() noexcept {", "gp::lib::enter(&::gp_init_all);"} {
		if !strings.Contains(s, want) {
			t.Errorf("init unit does not have %q:\n%s", want, s)
		}
	}
}

func TestCPPLibFiles(t *testing.T) {
	files := CPPLibFiles(CPPLibInfo{
		Name: "geo", Version: "1.2.3", Compiler: "GNU",
		Roots:   []RootPackage{{ImportPath: "example.com/geo/shapes", Name: "shapes", Header: "gpkg/example.com/geo/shapes/pkg.hpp", Namespace: "gpkg_example_dcom__geo__shapes"}, {ImportPath: "x/main", Name: "main", Header: "gpkg/x/main/pkg.hpp", Namespace: "gpkg_x__main"}},
		Compile: []string{"-std=c++20", "-fwrapv", "-iquote", "/src/dir"}, Link: []string{"-lsqlite3"},
	})
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Data)
	}
	conv := byPath["include/geo.hpp"]
	for _, want := range []string{"#include <gocpp/rt.hpp>", "#include \"gpkg/example.com/geo/shapes/pkg.hpp\"", "namespace gocpp_geo {", "namespace shapes = ::gpkg_example_dcom__geo__shapes;"} {
		if !strings.Contains(conv, want) {
			t.Errorf("geo.hpp does not have %q:\n%s", want, conv)
		}
	}
	if strings.Contains(conv, "namespace main =") {
		t.Errorf("an alias for package main:\n%s", conv)
	}
	cm := byPath["lib/cmake/geo/geoConfig.cmake"]
	for _, want := range []string{"gocpp::geo", "Threads::Threads", "-std=c++20", "SHELL:-iquote /src/dir", "-lsqlite3", "-ldl"} {
		if !strings.Contains(cm, want) {
			t.Errorf("geoConfig.cmake does not have %q:\n%s", want, cm)
		}
	}
	pc := byPath["lib/pkgconfig/geo.pc"]
	for _, want := range []string{"Version: 1.2.3", "-lgeo", "-pthread"} {
		if !strings.Contains(pc, want) {
			t.Errorf("geo.pc does not have %q:\n%s", want, pc)
		}
	}
}

func TestNameAndDedup(t *testing.T) {
	for in, want := range map[string]string{"geo": "geo", "my.lib/x": "my_lib_x", "a-b_c": "a-b_c", "": "lib", "é": "_"} {
		if got := Name(in); got != want {
			t.Errorf("Name(%q) = %q, want %q", in, got, want)
		}
	}
	if got := strings.Join(Dedup([]string{"-a", "-b", "-a", "-c", "-b"}), " "); got != "-a -b -c" {
		t.Errorf("Dedup = %q", got)
	}
}

func TestCheckSymbols(t *testing.T) {
	exports := sampleExports()
	exports[1].Name = "getpagesize"
	errs := CheckSymbols(exports, func(n string) bool { return n == "getpagesize" })
	if len(errs) != 1 || errs[0].Pos != exports[1].Pos || !strings.Contains(errs[0].Msg, "getpagesize is a symbol of the C library") {
		t.Errorf("errors: %v", errs)
	}
	if errs := CheckSymbols(sampleExports(), func(string) bool { return false }); len(errs) != 0 {
		t.Errorf("errors: %v", errs)
	}
}
