package probe

import (
	"context"
	"slices"
	"testing"
)

const arrayHeader = `#pragma once
namespace ui {
bool color3(const char* label, float col[3], int flags = 0);
bool begin(const char* name, bool* p_open = nullptr, int flags = 0);
bool spaced(const char* name, bool*   p_open   =   nullptr);
bool two(int v[2], const int w[2] = nullptr, void* a[1] = nullptr, int* z = 0);
bool unnamed(int*, const int[4]);
bool fixed(int* v, float f);
bool ends(const char* name, int* out) __attribute__((unused));
}
int c_sum(const int values[], int n);
int c_ptr(const int* values, int n);
int c_twice(int* a);
int c_twice(int a[2]);
#define OUT_ARRAY(name) name[2]
int c_macro(int OUT_ARRAY(pair));
struct Box {
  void set(float rgba[4], bool* p = nullptr, int* q=0, const int w[2] = nullptr);
  void plain(int* x, int y[]);
  void macro(int OUT_ARRAY(pair));
};
`

// TestArrayParams checks that a parameter written as an array is told from a pointer, for functions
// (the compiler's lookup dump) and methods (the JSON dump of the class).
func TestArrayParams(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "arr.hpp", arrayHeader)
	cfg := clangConfig(t, dir)
	addr := map[string]bool{}
	for _, name := range []string{"ui::color3", "ui::begin", "ui::spaced", "ui::two", "ui::unnamed",
		"ui::fixed", "ui::ends", "c_sum", "c_ptr", "c_twice", "c_macro"} {
		addr[name] = true
	}
	dc := DeclConfig{Includes: []string{`"./arr.hpp"`}, AddrFuncs: addr}
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{
		{DeclFunc, "ui::color3"}, {DeclFunc, "ui::begin"}, {DeclFunc, "ui::spaced"}, {DeclFunc, "ui::two"},
		{DeclFunc, "ui::unnamed"}, {DeclFunc, "ui::fixed"}, {DeclFunc, "ui::ends"}, {DeclFunc, "c_sum"},
		{DeclFunc, "c_ptr"}, {DeclFunc, "c_twice"}, {DeclFunc, "c_macro"}, {DeclType, "Box"},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		want []bool
	}{
		{"ui::color3", []bool{false, true, false}},
		{"ui::begin", []bool{false, false, false}},
		{"ui::spaced", []bool{false, true}}, // wide spaces around "=" are not provably a pointer
		{"ui::two", []bool{true, true, true, false}},
		{"ui::unnamed", []bool{false, true}},
		{"ui::fixed", []bool{false, false}},
		{"ui::ends", []bool{false, false}},
		{"c_sum", []bool{true, false}},
		{"c_ptr", []bool{false, false}},
		{"c_macro", []bool{true}}, // the macro's place is not read: an array
	}
	for _, tt := range tests {
		fs := tab.Funcs[tt.name]
		if fs == nil || len(fs.Overloads) != 1 {
			t.Errorf("%s: no overload set: %+v", tt.name, fs)
			continue
		}
		if got := fs.Overloads[0].ArrayParams; !slices.Equal(got, tt.want) {
			t.Errorf("%s: array parameters %v, want %v", tt.name, got, tt.want)
		}
	}
	twice := tab.Funcs["c_twice"]
	if twice == nil || len(twice.Overloads) != 1 || !slices.Equal(twice.Overloads[0].ArrayParams, []bool{true}) {
		t.Errorf("c_twice: the two declarations of one function must say array if any does: %+v", twice)
	}
	box := tab.Records["Box"]
	if box == nil {
		t.Fatalf("no record Box: %v", tab.Records)
	}
	methods := map[string][]bool{"set": {true, false, false, true}, "plain": {false, true}, "macro": {true}}
	for _, m := range box.Methods {
		want, ok := methods[m.Name]
		if !ok {
			continue
		}
		var got []bool
		for _, p := range m.Params {
			got = append(got, p.Array)
		}
		if !slices.Equal(got, want) {
			t.Errorf("Box::%s: array parameters %v, want %v", m.Name, got, want)
		}
	}
}

// TestArrayParamsAreAskedForTheFunctionsThatGetAnAddress checks that the compiler is asked only for
// the functions that a call gives the address of a variable, in the run that finds the overloads (no
// run of its own).
func TestArrayParamsAreAskedForTheFunctionsThatGetAnAddress(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "arr.hpp", arrayHeader)
	cfg := clangConfig(t, dir)
	dc := DeclConfig{Includes: []string{`"./arr.hpp"`}, AddrFuncs: map[string]bool{"c_sum": true}}
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{
		{DeclFunc, "c_sum"}, {DeclFunc, "c_ptr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := tab.Funcs["c_sum"].Overloads[0].ArrayParams; !slices.Equal(got, []bool{true, false}) {
		t.Errorf("c_sum: array parameters %v, want [true false]", got)
	}
	if got := tab.Funcs["c_ptr"].Overloads[0].ArrayParams; got != nil {
		t.Errorf("c_ptr: array parameters %v for a function that no call gives an address", got)
	}
	if tab.Runs != 1 {
		t.Errorf("%d compiler runs, want 1 (the lookups go with the run of the overload sets)", tab.Runs)
	}
}

const typedefHeader = `#pragma once
typedef int IVec[3];
typedef float vec3[3];
typedef float mat4[4][4];
typedef int* IPtr;
typedef const int* CIPtr;
typedef unsigned long* LPUL;
typedef IVec IVec2;
namespace ui {
typedef int Triple[3];
}
struct Hold {};
void tdef(IVec v);
void tdef_c(const IVec v);
void tdef_nested(IVec2 v, ui::Triple w);
void tdef_vec(vec3 v, float s);
void tdef_mat(mat4 m);
void tdef_ptr(IPtr p);
void tdef_cptr(const IPtr p, CIPtr q);
void tdef_lp(LPUL p, int* plain);
void tdef_unnamed(IVec, IPtr);
void tdef_comment(IVec /* a * b */ v);
struct Vec {
  void tdef(IVec v);
  void ptr(IPtr p, int* q);
  void mat(mat4 m);
};
`

// TestTypedefParams checks that a parameter written with the typedef of an array is an array
// (the compiler turns it into a pointer and keeps no name), and one written with the typedef of a
// pointer is a pointer, for functions and for methods.
func TestTypedefParams(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "td.hpp", typedefHeader)
	cfg := clangConfig(t, dir)
	names := []string{"tdef", "tdef_c", "tdef_nested", "tdef_vec", "tdef_mat", "tdef_ptr", "tdef_cptr", "tdef_lp",
		"tdef_unnamed", "tdef_comment"}
	addr := map[string]bool{}
	var queries []DeclQuery
	for _, name := range names {
		addr[name] = true
		queries = append(queries, DeclQuery{DeclFunc, name})
	}
	queries = append(queries, DeclQuery{DeclType, "Vec"})
	dc := DeclConfig{Includes: []string{`"./td.hpp"`}, AddrFuncs: addr}
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, queries)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		want []bool
	}{
		{"tdef", []bool{true}},
		{"tdef_c", []bool{true}},
		{"tdef_nested", []bool{true, true}},
		{"tdef_vec", []bool{true, false}},
		{"tdef_mat", []bool{true}},
		{"tdef_ptr", []bool{false}},
		{"tdef_cptr", []bool{false, false}},
		{"tdef_lp", []bool{false, false}},
		{"tdef_unnamed", []bool{true, false}},
		{"tdef_comment", []bool{true}}, // the star is in a comment
	}
	for _, tt := range tests {
		fs := tab.Funcs[tt.name]
		if fs == nil || len(fs.Overloads) != 1 {
			t.Errorf("%s: no overload set: %+v", tt.name, fs)
			continue
		}
		if got := fs.Overloads[0].ArrayParams; !slices.Equal(got, tt.want) {
			t.Errorf("%s: array parameters %v, want %v", tt.name, got, tt.want)
		}
	}
	vec := tab.Records["Vec"]
	if vec == nil {
		t.Fatalf("no record Vec: %v", tab.Records)
	}
	methods := map[string][]bool{"tdef": {true}, "ptr": {false, false}, "mat": {true}}
	for _, m := range vec.Methods {
		want, ok := methods[m.Name]
		if !ok {
			continue
		}
		var got []bool
		for _, p := range m.Params {
			got = append(got, p.Array)
		}
		if !slices.Equal(got, want) {
			t.Errorf("Vec::%s: array parameters %v, want %v", m.Name, got, want)
		}
	}
}
