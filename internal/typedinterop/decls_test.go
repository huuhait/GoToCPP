package typedinterop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

const physHeader = `#pragma once
#include <vector>
#include <string>
#include <cstddef>
typedef struct { int w, h; } Rect;
typedef unsigned char Byte;
namespace phys {
struct Vec { double x, y; double len() const; };
enum class Mode : unsigned char { Off, On = 3 };
enum Level { Low = -1, High = 5 };
struct Body { Vec pos; Vec vel; Mode mode; std::vector<Vec> trail; std::string name; size_t id; Body *next; Vec *begin; Level lv; Byte tag; };
struct Derived : Body { int extra; };
Body make(double x, double y);
Vec add(const Vec& a, const Vec& b);
Vec add(const Vec& a, double k);
}
Rect grow(Rect r, int by);
`

const physProgram = `package main

import (
	"fmt"
	lib "./lib.hpp"
)

func main() {
	b := phys.make(1, 2)
	p := b.pos
	n := b.id
	v := phys.add(b.pos, b.vel)
	w := phys.add(b.pos, 2.5)
	m := b.mode
	t := b.trail.size()
	var r lib.Rect
	g := lib.grow(r, 2)
	var d phys.Derived
	e := d.extra
	l := d.lv
	x := d.pos.x
	c := b.tag
	fmt.Println(p, n, v, w, m, t, g, e, l, x, c)
}
`

func physReport(t testing.TB, opts Options) *Report {
	t.Helper()
	opts.Decls = true
	return analyze(t, physProgram, map[string]string{"lib.hpp": physHeader}, opts)
}

func value(t testing.TB, rep *Report, site string) ftype.Value {
	t.Helper()
	r := find(t, rep, site)
	return rep.Values[r.ID]
}

func TestAnalyzeDecls(t *testing.T) {
	rep := physReport(t, Options{})
	if rep.Decls == nil || rep.Mapper == nil || len(rep.Values) != len(rep.Table.Results) {
		t.Fatalf("no declarations: %+v", rep.Decls)
	}
	for site, want := range map[string]string{
		"C.phys.make(1, 2)":        "value phys.Body",
		"b.pos":                    "assignable phys.Vec",
		"b.id":                     "assignable C.ulong (size_t)",
		"C.phys.add(b.pos, b.vel)": "value phys.Vec",
		"C.phys.add(b.pos, 2.5)":   "value phys.Vec",
		"b.mode":                   "assignable phys.Mode",
		"b.trail.size()":           "value C.ulong",
		"C.grow(r, 2)":             "value Rect",
		"d.extra":                  "assignable C.int",
		"d.lv":                     "assignable phys.Level",
		"d.pos.x":                  "assignable C.double",
		"b.tag":                    "assignable C.uchar",
	} {
		if got := value(t, rep, site).String(); got != want {
			t.Errorf("%s: got %q, want %q", site, got, want)
		}
	}
	// Records and enums of the namespace, with the types of their members resolved.
	body := rep.Decls.Records["phys::Body"]
	if body == nil || body.Scope != "phys" || !body.Complete || len(body.Fields) != 10 {
		t.Fatalf("Body: %+v (%v)", body, rep.Decls.Errors)
	}
	m := rep.Mapper
	bt := value(t, rep, "C.phys.make(1, 2)").Type
	ms := m.MembersOf(bt)
	if ms == nil {
		t.Fatal("no members of Body")
	}
	for name, want := range map[string]string{
		"pos": "phys.Vec", "trail": "std.vector[phys.Vec]", "name": "std.string", "id": "C.ulong", "next": "*phys.Body",
		"begin": "*phys.Vec", "lv": "phys.Level", "tag": "C.uchar", "mode": "phys.Mode",
	} {
		mem := ms.Lookup(name)
		if mem == nil || mem.Kind != "field" || m.TypeString(mem.Type.Type) != want {
			t.Errorf("Body.%s: %+v, want %s", name, mem, want)
		}
	}
	// The derived class has the members of Body.
	dt := value(t, rep, "C.phys.Derived").Type
	if mem := m.MembersOf(dt).Lookup("pos"); mem == nil || mem.Of != "phys::Body" {
		t.Errorf("Derived.pos: %+v", mem)
	}
	// Enumerators.
	mode := value(t, rep, "b.mode")
	if cs := m.Enumerators(mode.Type); len(cs) != 2 || cs[1].Val().String() != "3" {
		t.Errorf("Mode: %v", cs)
	}
	// The overload set of a function, with the signatures resolved in the namespace.
	add := rep.Decls.Funcs["phys::add"]
	if add == nil || len(add.Overloads) != 2 || add.Scope != "phys" {
		t.Fatalf("add: %+v (%v)", add, rep.Decls.Errors)
	}
	sig := m.MapIn(add.Overloads[1].Sig.T, add.Scope)
	if got := m.TypeString(sig.Type); got != "func(phys.Vec, C.double) phys.Vec" || sig.Partial {
		t.Errorf("add(Vec, double): %s %+v", got, sig)
	}
	// Typedef of an anonymous struct, a typedef of a builtin.
	if r := rep.Decls.Records["Rect"]; r == nil || len(r.Fields) != 2 {
		t.Errorf("Rect: %+v", r)
	}
}

// By default nothing changes: no declarations, no extra compiler runs, the same table.
func TestDeclsOffByDefault(t *testing.T) {
	clang := tsupport.Clang(t)
	dir := t.TempDir()
	write(t, dir, "lib.hpp", physHeader)
	write(t, dir, "main.gp", physProgram)
	// (a header written in the last two seconds is not remembered: the cache's racy rule)
	defer probe.SetRacyWindow(0)()
	p, fset := tsupport.CheckDir(t, dir, "main.gp")
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatal(errs)
	}
	rn := new(probe.Runner)
	off, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: rn})
	if err != nil {
		t.Fatal(err)
	}
	if off.Decls != nil || off.Mapper != nil || off.Values != nil || rn.DeclRuns() != 0 {
		t.Errorf("declarations without Decls: %+v, %d runs", off.Decls, rn.DeclRuns())
	}
	on, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: rn, Decls: true})
	if err != nil {
		t.Fatal(err)
	}
	if off.Table.Source.Text != on.Table.Source.Text || off.Table.Key != on.Table.Key || !on.Table.Cached {
		t.Error("asking for declarations changed the probe")
	}
	if rn.Runs() != 1 || rn.DeclRuns() == 0 {
		t.Errorf("%d probe runs, %d declaration runs", rn.Runs(), rn.DeclRuns())
	}
	// A second time everything is remembered: no compiler run at all.
	runs := rn.DeclRuns()
	again, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: rn, Decls: true})
	if err != nil {
		t.Fatal(err)
	}
	if rn.Runs() != 1 || rn.DeclRuns() != runs || again.Decls.Runs != 0 {
		t.Errorf("a warm analysis ran the compiler: %d probe runs, %d declaration runs (was %d), %d in the table", rn.Runs(), rn.DeclRuns(), runs, again.Decls.Runs)
	}
	for i := range on.Values {
		if on.Values[i].String() != again.Values[i].String() {
			t.Errorf("value %d changed: %s / %s", i, on.Values[i], again.Values[i])
		}
	}
}

// Editing a header the declarations came from invalidates them (by content).
func TestDeclsHeaderEdit(t *testing.T) {
	clang := tsupport.Clang(t)
	dir := t.TempDir()
	write(t, dir, "lib.hpp", physHeader)
	write(t, dir, "main.gp", physProgram)
	p, fset := tsupport.CheckDir(t, dir, "main.gp")
	rn := new(probe.Runner)
	first, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: rn, Decls: true})
	if err != nil {
		t.Fatal(err)
	}
	vec := first.Decls.Records["phys::Vec"]
	if vec == nil || len(vec.Fields) != 2 {
		t.Fatalf("Vec: %+v", vec)
	}
	write(t, dir, "lib.hpp", strings.Replace(physHeader, "struct Vec { double x, y;", "struct Vec { double x, y, z;", 1))
	p2, fset2 := tsupport.CheckDir(t, dir, "main.gp")
	second, err := Analyze(context.Background(), p2, fset2, Options{Clang: clang, Runner: rn, Decls: true})
	if err != nil {
		t.Fatal(err)
	}
	vec = second.Decls.Records["phys::Vec"]
	if vec == nil || len(vec.Fields) != 3 || vec.Size != 24 {
		t.Errorf("after the edit: %+v", vec)
	}
	if second.Decls.Runs == 0 {
		t.Error("the edit did not run the compiler")
	}
}

func write(t testing.TB, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The standard library through the mapper: T& results are assignable, the members of a
// std::vector come from its declaration, std::pair has its fields, enumerators of an enum
// class are constants.
func TestAnalyzeDeclsStd(t *testing.T) {
	rep := analyze(t, `package main

import (
	"fmt"
	"<vector>"
	"<utility>"
	"<tuple>"
	cstdio "<cstdio>"
)

func main() {
	v := std.vector[int]{1, 2, 3}
	f := v.front()
	v.front() = 5
	p := std.make_pair(1, 2.5)
	a := p.first
	t := std.make_tuple(1, 2)
	g := std.get[0](t)
	h := cstdio.fopen("x", "r")
	e := cstdio.EOF
	fmt.Println(f, a, g, h, e)
}
`, nil, Options{Decls: true})
	for site, want := range map[string]string{
		"v.front()":             "assignable C.long (int64_t)", // never "value": T& is addressable
		"p.first":               "assignable C.int",
		"C.std.get[0](t)":       "assignable C.int",
		"C.fopen(\"x\", \"r\")": "value *_IO_FILE (*FILE)",
		"C.EOF":                 "value C.int",
	} {
		got := value(t, rep, site).String()
		if site == "v.front()" {
			got = strings.Replace(got, "C.long (int64_t)", "C.long", 1)
			want = "assignable C.long"
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", site, got, want)
		}
	}
	if v := value(t, rep, "C.EOF"); v.ConstValue == nil || v.ConstValue.Int.Int64() != -1 {
		t.Errorf("EOF: %+v", v.ConstValue)
	}
	m := rep.Mapper
	vec := value(t, rep, "local:v")
	if vec.String() != "type std.vector[C.long]" {
		t.Fatalf("v: %s", vec)
	}
	ms := m.MembersOf(vec.Type)
	if ms == nil {
		t.Fatalf("no members of std::vector<long>: %v", rep.Decls.Errors)
	}
	for _, name := range []string{"push_back", "size", "front", "at", "begin", "operator[]", "clear"} {
		if mem := ms.Lookup(name); mem == nil || mem.Kind != "method" || mem.Access != probe.Public {
			t.Errorf("vector.%s: %+v", name, mem)
		}
	}
	if pb := ms.Lookup("push_back"); pb == nil || len(pb.Methods) != 2 {
		t.Errorf("push_back overloads: %+v", pb)
	} else if got := m.TypeString(pb.Methods[0].Sig.Type); got != "func(C.long)" {
		t.Errorf("push_back(const value_type&): %s", got)
	}
	if mem := ms.Lookup("size"); mem == nil || m.TypeString(mem.Methods[0].Sig.Type) != "func() C.ulong" || !mem.Methods[0].Const {
		t.Errorf("size: %+v", mem)
	}
	if ms.Lookup("_M_impl") != nil && ms.Lookup("_M_impl").Access == probe.Public {
		t.Error("a private member is public")
	}
	if ms.Lookup("no_such_member") != nil {
		t.Error("a member from nowhere")
	}
	// std::pair<int, double> has the fields first and second, and std::fopen's FILE is
	// known by the name the program uses.
	pair := value(t, rep, "local:p")
	if pm := m.MembersOf(pair.Type); pm == nil || pm.Lookup("first") == nil || pm.Lookup("second") == nil {
		t.Errorf("pair members: %v", pm)
	}
	if h := value(t, rep, "C.fopen(\"x\", \"r\")"); h.Display() != "*FILE" || h.Elem == nil || h.Elem.Alias != "FILE" {
		t.Errorf("fopen result is %q, want *FILE", h.Display())
	}
}

// The names of the functions that a program calls: free functions and function templates of
// namespaces and of the global scope; not the members of a class, not cgo's own names.
func TestDeclsFuncNames(t *testing.T) {
	rep := analyze(t, `package main

import (
	"fmt"
	cstdio "<cstdio>"
	"<algorithm>"
	"<thread>"
	"<chrono>"
	lib "./lib.hpp"
)

func main() {
	f := cstdio.fopen("x", "r")
	m := std.min(1, 2)
	n := std.thread.hardware_concurrency()
	t := std.chrono.steady_clock.now()
	b := phys.make(1, 2)
	v := phys.add(b.pos, b.vel)
	g := lib.grow(lib.Rect{}, 2)
	s := C.CString("x")
	fmt.Println(f, m, n, t, v, g, s)
}
`, map[string]string{"lib.hpp": physHeader}, Options{})
	got := probe.FuncNames(rep.Set, rep.Kinds)
	want := []string{"fopen", "grow", "phys::add", "phys::make", "std::min"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("function names %v, want %v", got, want)
	}
}
