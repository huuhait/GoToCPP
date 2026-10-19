package probe

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

const declsHeader = `#pragma once
#include <cstdio>
#include <vector>
#include <string>
namespace geo {
struct Vec { double x, y; unsigned flags : 3; static int count; double len() const; Vec operator+(const Vec&) const; Vec(double, double); };
class Shape { public: virtual ~Shape(); virtual double area() const = 0; int id; protected: int prot; private: int priv; };
struct Body { Vec pos; Vec *next; const char *name; int arr[4]; void (*cb)(int, Vec); };
enum class Kind : unsigned char { A, B = 5, C };
enum Old { O1 = -1, O2, O3 = 100 };
int f(int); long f(long, const char*); template <class T> T g(T, int = 3);
}
typedef struct { int w, h; } Rect;
typedef struct Pt { int x; } Pt;
union U { int i; float f; };
struct Opaque;
struct Box : geo::Shape {
  explicit Box(int); Box(const Box&) = delete; ~Box() override;
  double area() const override final; operator bool() const; static constexpr int K = 5; mutable int m;
  template <class T> T get() const; int g(int a, int b = 3) const noexcept; int h(...) volatile &&;
  typedef long handle; struct Inner { int z; }; enum E { E1 }; union { int ui; float uf; };
  handle h1; Inner in; E e; Box& operator=(Box&&) = default; int arr[4]; const char* name;
};
extern "C" { int cfun(Rect r, Pt *p); }
typedef unsigned int Id;
struct T { int a; };
namespace nn { struct T { long b; }; }
struct Nest { int tag; union { struct { int lo, hi; }; long both; }; };
struct Mixed { struct { int x, y; } pos; union { int ival; double dval; }; struct { char c; } tail; };
struct TD1 { typedef struct { int a; } Inner; Inner in; int x; };
struct TD2 { typedef union { int i; float f; } U; U u; int y; };
struct TD3 { static constexpr struct { int a; } cfg{}; int z; };
struct TD4 { using X = struct { int q; }; X xx; int w; };
struct TD5 { struct { int p, q; } pq; typedef struct { char c; } Ch; Ch ch; union { int ui; float uf; }; int last; };
struct Deep { union { struct { union { char c; short s; }; int i; }; double d; }; int after; };
template <class T> struct Wrap { T v; T get() const; };
extern template struct Wrap<int>;
template <class T, int N> struct Buf { T data[N]; static constexpr int size = N; };
`

func declsSetup(t testing.TB) (Config, DeclConfig) {
	dir := t.TempDir()
	writeHeader(t, dir, "decls.hpp", declsHeader)
	return clangConfig(t, dir), DeclConfig{Includes: []string{`"./decls.hpp"`}}
}

func TestDeclsRecords(t *testing.T) {
	cfg, dc := declsSetup(t)
	rn := new(Runner)
	tab, err := rn.Decls(context.Background(), cfg, dc, []DeclQuery{
		{DeclType, "geo::Vec"}, {DeclType, "geo::Shape"}, {DeclType, "geo::Body"}, {DeclType, "Rect"}, {DeclType, "Pt"}, {DeclType, "U"},
		{DeclType, "Opaque"}, {DeclType, "Box"}, {DeclType, "geo::Kind"}, {DeclType, "geo::Old"}, {DeclType, "Nope"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.HeaderErrors != nil {
		t.Errorf("header errors %+v", tab.HeaderErrors)
	}
	vec := tab.Records["geo::Vec"]
	if vec == nil {
		t.Fatalf("no geo::Vec: %v", tab.Errors)
	}
	if vec.Tag != "struct" || !vec.Complete || vec.Size != 24 && vec.Size != 32 || vec.Scope != "geo" {
		t.Errorf("Vec: %+v", vec)
	}
	if got := fieldList(vec); got != "x:double y:double flags:unsigned int/3" {
		t.Errorf("Vec fields %q", got)
	}
	if len(vec.Statics) != 1 || vec.Statics[0].Name != "count" || vec.Statics[0].Type.Text != "int" {
		t.Errorf("Vec statics %+v", vec.Statics)
	}
	if got := methodList(vec); got != "len:double () const;operator+:Vec (const Vec &) const;Vec:void (double, double)" {
		t.Errorf("Vec methods %q", got)
	}
	if ms := vec.MethodsNamed("operator+"); len(ms) != 1 || ms[0].Kind != KindOperator || ms[0].Sig.T == nil || ms[0].Sig.T.Params[0].Elem.Last() == "" {
		t.Errorf("operator+ %+v", ms)
	}
	if !vec.Traits["standardLayout"] {
		t.Errorf("traits %v", vec.Traits)
	}

	shape := tab.Records["geo::Shape"]
	if shape == nil || shape.Tag != "class" || !shape.Traits["abstract"] || !shape.Traits["polymorphic"] {
		t.Fatalf("Shape %+v (%v)", shape, tab.Errors)
	}
	if got := accessList(shape); got != "id:public prot:protected priv:private" {
		t.Errorf("Shape access %q", got)
	}
	if ms := shape.MethodsNamed("area"); len(ms) != 1 || !ms[0].Virtual || !ms[0].Pure || ms[0].Access != Public {
		t.Errorf("area %+v", ms)
	}

	body := tab.Records["geo::Body"]
	if body == nil || fieldList(body) != "pos:Vec next:Vec * name:const char * arr:int[4] cb:void (*)(int, Vec)" {
		t.Fatalf("Body %v %v", body, tab.Errors)
	}
	// Member types are written relative to the scope, which the record gives.
	if body.Scope != "geo" || body.Field("pos").Type.T.Last() != "Vec" || len(body.Field("pos").Type.T.Path) != 1 {
		t.Errorf("Body.pos %+v", body.Field("pos").Type)
	}

	rect := tab.Records["Rect"]
	if rect == nil || fieldList(rect) != "w:int h:int" || rect.Size != 8 {
		t.Errorf("Rect %+v (%v)", rect, tab.Errors)
	}
	if pt := tab.Records["Pt"]; pt == nil || fieldList(pt) != "x:int" {
		t.Errorf("Pt %+v", pt)
	}
	if u := tab.Records["U"]; u == nil || u.Tag != "union" || fieldList(u) != "i:int f:float" || u.Size != 4 {
		t.Errorf("U %+v", u)
	}
	op := tab.Records["Opaque"]
	if op == nil || op.Complete || op.Size != -1 || len(op.Fields) != 0 {
		t.Errorf("Opaque %+v (%v)", op, tab.Errors)
	}

	box := tab.Records["Box"]
	if box == nil {
		t.Fatalf("no Box: %v", tab.Errors)
	}
	if len(box.Bases) != 1 || box.Bases[0].Access != Public || box.Bases[0].Type.T == nil || box.Bases[0].Type.T.QualName() != "geo::Shape" {
		t.Errorf("Box bases %+v", box.Bases)
	}
	// typedef handle is long, and the nested types are qualified.
	if f := box.Field("h1"); f == nil || f.Type.T == nil || f.Type.T.String() != "long" {
		t.Errorf("h1 %+v", f)
	}
	if f := box.Field("in"); f == nil || f.Type.T == nil || f.Type.T.String() != "Box::Inner" {
		t.Errorf("in %+v", f)
	}
	if f := box.Field("ui"); f == nil || !f.Anon || !f.Union {
		t.Errorf("anonymous union member %+v", f)
	}
	if got := strings.Join(box.Nested, ","); got != "Inner,E" {
		t.Errorf("nested %q", got)
	}
	var sawDeleted, sawTemplate, sawOverride, sawVariadic bool
	for _, m := range box.Methods {
		switch {
		case m.Kind == KindCtor && m.Deleted:
			sawDeleted = true
		case m.Name == "get" && m.Template:
			sawTemplate = true
		case m.Name == "area" && m.Override && m.Final:
			sawOverride = true
		case m.Name == "h" && m.Variadic:
			sawVariadic = true
		}
	}
	if !sawDeleted || !sawTemplate || !sawOverride || !sawVariadic {
		t.Errorf("methods: deleted %v template %v override %v variadic %v", sawDeleted, sawTemplate, sawOverride, sawVariadic)
	}
	if ms := box.MethodsNamed("g"); len(ms) != 1 || len(ms[0].Params) != 2 || ms[0].Params[0].Name != "a" || !ms[0].Params[1].Default || ms[0].Sig.T.Noexcept == "" {
		t.Errorf("g %+v", ms)
	}
	if st := box.Statics; len(st) != 1 || !st[0].Constexpr {
		t.Errorf("statics %+v", st)
	}

	kind := tab.Enums["geo::Kind"]
	if kind == nil || !kind.Scoped || kind.Underlying.Text != "unsigned char" || enumList(kind) != "A=0 B=5 C=6" || kind.Size != 1 {
		t.Errorf("Kind %+v (%v)", kind, tab.Errors)
	}
	old := tab.Enums["geo::Old"]
	if old == nil || old.Scoped || old.Underlying.Text != "int" || enumList(old) != "O1=-1 O2=0 O3=100" {
		t.Errorf("Old %+v", old)
	}
	if e := tab.Errors["type Nope"]; !strings.Contains(e, "Nope") {
		t.Errorf("Nope: %q", e)
	}
	if tab.Runs < 5 {
		t.Errorf("only %d runs", tab.Runs)
	}
	// JSON is stable and complete.
	if _, err := json.Marshal(tab); err != nil {
		t.Error(err)
	}
}

func fieldList(r *Record) string {
	var s []string
	for _, f := range r.Fields {
		x := f.Name + ":" + f.Type.Text
		if f.Bits > 0 {
			x += "/" + itoa(f.Bits)
		}
		s = append(s, x)
	}
	return strings.Join(s, " ")
}

func accessList(r *Record) string {
	var s []string
	for _, f := range r.Fields {
		s = append(s, f.Name+":"+string(f.Access))
	}
	return strings.Join(s, " ")
}

func methodList(r *Record) string {
	var s []string
	for _, m := range r.Methods {
		if m.Implicit {
			continue
		}
		s = append(s, m.Name+":"+m.Sig.Text)
	}
	return strings.Join(s, ";")
}

func enumList(e *Enum) string {
	var s []string
	for _, v := range e.Values {
		s = append(s, v.Name+"="+v.Value.String())
	}
	return strings.Join(s, " ")
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestDeclsStd(t *testing.T) {
	cfg, dc := declsSetup(t)
	rn := new(Runner)
	tab, err := rn.Decls(context.Background(), cfg, dc, []DeclQuery{
		{DeclType, "std::vector<long>"}, {DeclType, "std::basic_string<char>"}, {DeclType, "_IO_FILE"},
		{DeclFunc, "geo::f"}, {DeclFunc, "geo::g"}, {DeclFunc, "fopen"}, {DeclFunc, "cfun"}, {DeclFunc, "std::min"}, {DeclFunc, "geo::nothing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(tab, "", " ")
	if os.Getenv("DECLS_DUMP") != "" {
		os.WriteFile(os.Getenv("DECLS_DUMP"), b, 0o644)
	}
	vec := tab.Records["std::vector<long>"]
	if vec == nil {
		t.Fatalf("no vector: %v", tab.Errors)
	}
	// std::basic_string<char> has more declarations than the limit: no stub of it is reported.
	if tab.Records["std::basic_string<char>"] != nil || !strings.Contains(tab.Errors["type std::basic_string<char>"], "larger than the limit") {
		t.Errorf("basic_string: %v %v", tab.Records["std::basic_string<char>"], tab.Errors["type std::basic_string<char>"])
	}
	t.Logf("vector: %d fields %d methods %d bases, size %d, types %v", len(vec.Fields), len(vec.Methods), len(vec.Bases), vec.Size, vec.Types["size_type"])
	for _, f := range tab.Funcs {
		t.Logf("%s: %+v", f.Name, f.Overloads)
	}
	t.Logf("errors %v", tab.Errors)
	t.Logf("runs %d", tab.Runs)
}

// Anonymous unions and structs inside anonymous unions are members of the record, at any depth.
func TestDeclsNestedAnonymous(t *testing.T) {
	cfg, dc := declsSetup(t)
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "Nest"}, {DeclType, "Deep"}, {DeclType, "Mixed"}})
	if err != nil {
		t.Fatal(err)
	}
	n, d := tab.Records["Nest"], tab.Records["Deep"]
	if n == nil || d == nil {
		t.Fatalf("%v", tab.Errors)
	}
	if !n.Complete || n.Partial != "" || n.Size != 16 {
		t.Errorf("Nest: %+v", n)
	}
	if got := fieldList(n); got != "tag:int lo:int hi:int both:long" {
		t.Errorf("Nest fields %q", got)
	}
	for _, name := range []string{"lo", "hi", "both"} {
		if f := n.Field(name); f == nil || !f.Anon || !f.Union {
			t.Errorf("Nest.%s: %+v", name, f)
		}
	}
	if f := n.Field("tag"); f.Anon || f.Union {
		t.Errorf("Nest.tag: %+v", f)
	}
	if got := fieldList(d); got != "c:char s:short i:int d:double after:int" || !d.Complete || d.Size != 16 {
		t.Errorf("Deep fields %q complete %v size %d", got, d.Complete, d.Size)
	}
	// A named member of an unnamed struct type is a field; the unnamed struct is not an anonymous member.
	mx := tab.Records["Mixed"]
	if mx == nil || !mx.Complete || fieldList(mx) != "pos:struct (unnamed struct) ival:int dval:double tail:struct (unnamed struct)" {
		t.Errorf("Mixed: %+v", mx)
	}
	compileVerify(t, cfg.Dir, verifyProgram(tab, "decls.hpp"))
}

// A typedef, a using or a static member of an unnamed struct or union is not an anonymous member:
// the record keeps all its fields and stays complete (the compilers agree: see the verifier).
func TestDeclsUnnamedTypedefs(t *testing.T) {
	cfg, dc := declsSetup(t)
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "TD1"}, {DeclType, "TD2"}, {DeclType, "TD3"}, {DeclType, "TD4"}, {DeclType, "TD5"}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"TD1": "in x", "TD2": "u y", "TD3": "z", "TD4": "xx w", "TD5": "pq ch ui uf last"} {
		r := tab.Records[name]
		if r == nil || !r.Complete || r.Partial != "" {
			t.Errorf("%s: %+v (%v)", name, r, tab.Errors)
			continue
		}
		var names []string
		for _, f := range r.Fields {
			names = append(names, f.Name)
		}
		if strings.Join(names, " ") != want {
			t.Errorf("%s: fields %v, want %s", name, names, want)
		}
	}
	if tab.Records["TD3"] != nil && len(tab.Records["TD3"].Statics) != 1 {
		t.Errorf("TD3 statics: %+v", tab.Records["TD3"].Statics)
	}
	compileVerify(t, cfg.Dir, verifyProgram(tab, "decls.hpp"))
}
