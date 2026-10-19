package ftype

import (
	"math/big"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// result builds a probe result the way the probe's reader would for type text.
func result(typ, written string, form probe.Form) probe.Result {
	r := probe.Result{OK: true, Form: form, Written: written}
	r.Type, r.Base = typ, typ
	if form == probe.FormExpr {
		r.Category = probe.CatPRValue
		switch {
		case strings.HasSuffix(typ, " &&"):
			r.Base, r.Category = strings.TrimSuffix(typ, " &&"), probe.CatXValue
		case strings.HasSuffix(typ, " &"):
			r.Base, r.Category = strings.TrimSuffix(typ, " &"), probe.CatLValue
		}
		if i := strings.Index(typ, "(&)"); i >= 0 {
			r.Base, r.Category = typ[:i]+typ[i+3:], probe.CatLValue
		}
		r.Const = strings.HasPrefix(r.Base, "const ")
	}
	return r
}

func TestValue(t *testing.T) {
	m := New(table(), LP64)
	for _, c := range []struct {
		r    probe.Result
		want string
	}{
		{result("unsigned long", "size_t", probe.FormExpr), "value C.ulong (size_t)"},
		{result("long &", "", probe.FormExpr), "assignable C.long"},
		{result("const long &", "", probe.FormExpr), "lvalue C.long"},
		{result("long &&", "", probe.FormExpr), "xvalue C.long"},
		{result("const char *", "", probe.FormExpr), "value *C.char"},
		{result("_IO_FILE *", "FILE *", probe.FormExpr), "value *_IO_FILE (*FILE)"},
		{result("std::vector<long> &", "value_type &", probe.FormExpr), "assignable std.vector[C.long] (members unknown)"}, // a member typedef is not a typedef of the type
		{result("geo::Vec", "Alias", probe.FormExpr), "value geo.Vec"},
		{result("geo::Vec", "", probe.FormType), "type geo.Vec"},
		{result("geo::Vec &", "", probe.FormExpr), "assignable geo.Vec"},
		{result("int (&)[4]", "", probe.FormExpr), "lvalue [4]C.int"},
		{result("void", "", probe.FormExpr), "value void"},
		{result("void (*)(int)", "", probe.FormExpr), "value func(C.int)"},
		{result("int (int)", "", probe.FormExpr), "value func(C.int) C.int"}, // a function name is not assignable
		{result("int S::*", "", probe.FormExpr), "opaque: pointer to member"},
		{result("not a ((type", "", probe.FormExpr), "opaque: cannot read the C++ type \"not a ((type\" at 4: unexpected \"a ((type\" after the type"},
		{probe.Result{Skipped: "incomplete: a Go struct"}, "opaque: not probed: incomplete: a Go struct"},
		{probe.Result{Error: "no matching function"}, "opaque: the probe failed: no matching function"},
	} {
		got := m.Value(&c.r).String()
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.r.Type, got, c.want)
		}
	}
	// Constants keep their value.
	r := result("const int", "", probe.FormExpr)
	r.Value = &probe.Const{Kind: "int", Int: big.NewInt(8192)}
	if v := m.Value(&r); v.ConstValue == nil || v.ConstValue.Int.Int64() != 8192 || v.Assignable {
		t.Errorf("%+v", v)
	}
	// A const array and a function are not assignable, a void is not a value.
	for _, ty := range []string{"int (&)[4]", "void (&)(int)"} {
		r := result(ty, "", probe.FormExpr)
		if v := m.Value(&r); v.Assignable {
			t.Errorf("%s is assignable", ty)
		}
	}
}

func TestCompat(t *testing.T) {
	m := New(table(), LP64)
	val := func(typ string) Value {
		r := result(typ, "", probe.FormExpr)
		return m.Value(&r)
	}
	for _, c := range []struct {
		typ, target string
		want        Verdict
	}{
		{"int", "int", Yes}, {"int", "float64", Yes}, {"double", "int", Yes}, {"int", "bool", Yes}, {"bool", "int", Yes},
		{"unsigned long", "uint64", Yes}, {"int", "string", Unknown}, {"int", "*int", No}, {"int", "[]int", No},
		{"int", "map[string]int", No}, {"int", "func()", No},
		{"void", "int", No}, {"void", "string", No},
		{"char *", "int", No}, {"char *", "bool", Yes}, {"char *", "*uint8", Yes}, {"char *", "[]byte", No},
		{"void (*)(int)", "func(int)", Yes}, {"void (*)(int)", "int", No},
		{"geo::Vec", "int", Unknown}, {"geo::Vec", "string", Unknown}, {"geo::Vec", "mytype", Unknown},
		{"Old", "int", Yes}, {"Old", "*int", No}, {"geo::Kind", "int", Unknown}, {"geo::Kind", "string", No},
		{"gocpp_probe_internal_pr::string", "string", Yes}, {"gocpp_probe_internal_pr::string", "int", No},
		{"gocpp_probe_internal_pr::slice<long>", "[]int", No}, // []C.long is not []int
		{"int[4]", "int", No}, {"int[4]", "string", No},
		{"int", "", Unknown}, {"int", "invalid type", Unknown}, {"int", "SomeNamedType", Unknown},
		{"__int128", "int", Unknown},
	} {
		if got := val(c.typ).Compat(c.target); got != c.want {
			t.Errorf("%s -> %s: got %v, want %v", c.typ, c.target, got, c.want)
		}
	}
}

// A null pointer constant (NULL is a long 0) goes to a pointer or a function pointer; nothing
// else that is a number does, and a value that is not constant does not.
func TestCompatNull(t *testing.T) {
	m := New(table(), LP64)
	val := func(typ string, c *probe.Const) Value {
		r := result(typ, "", probe.FormExpr)
		r.Value = c
		return m.Value(&r)
	}
	zero := &probe.Const{Kind: "int", Int: big.NewInt(0)}
	one := &probe.Const{Kind: "int", Int: big.NewInt(1)}
	for _, c := range []struct {
		name   string
		v      Value
		target string
		want   Verdict
	}{
		{"NULL to char*", val("long", zero), "*C.char", Yes}, {"NULL to *int32", val("long", zero), "*int32", Yes},
		{"NULL to func", val("long", zero), "func()", Yes}, {"NULL to unsafe.Pointer", val("long", zero), "unsafe.Pointer", Unknown},
		{"NULL to slice", val("long", zero), "[]int", Unknown}, {"NULL to map", val("long", zero), "map[string]int", Unknown},
		{"NULL to int", val("long", zero), "int", Yes}, {"NULL to bool", val("long", zero), "bool", Yes},
		{"int 0 to pointer", val("int", zero), "*int", Yes}, {"unsigned 0 to pointer", val("unsigned long", zero), "*int", Yes},
		{"1 to pointer", val("long", one), "*int", No}, {"1 to func", val("long", one), "func()", No}, {"1 to map", val("long", one), "map[int]int", No},
		{"not constant to pointer", val("long", nil), "*int", No},
		{"false to pointer", val("bool", &probe.Const{Kind: "bool", Int: big.NewInt(0)}), "*int", No},
		{"0.0 to pointer", val("double", &probe.Const{Kind: "float"}), "*int", No},
		{"an enumerator 0 to pointer", val("Old", &probe.Const{Kind: "int", Int: big.NewInt(0)}), "*int", No},
		{"nullptr to pointer", val("std::nullptr_t", nil), "*int", Yes}, {"nullptr to func", val("std::nullptr_t", nil), "func()", Yes},
		{"nullptr to slice", val("std::nullptr_t", nil), "[]int", Unknown}, {"nullptr to int", val("std::nullptr_t", nil), "int", No},
	} {
		if got := c.v.Compat(c.target); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNeeds(t *testing.T) {
	m := New(nil, LP64)
	rs := []probe.Result{
		result("geo::Vec *", "Vec *", probe.FormExpr),
		result("std::vector<geo::Body>", "", probe.FormType),
		result("std::pair<int, Old>", "", probe.FormExpr),
		result("_IO_FILE *", "FILE *", probe.FormExpr),
		result("long", "int64_t", probe.FormExpr),
		result("__gnu_cxx::__normal_iterator<long *, std::vector<long>>", "iterator", probe.FormExpr),
		result("gocpp_probe_internal_pr::slice<Rect>", "", probe.FormExpr),
		{OK: false, Error: "x", Type: "ignored::T"},
	}
	var got []string
	for _, q := range m.Needs(rs) {
		if q.Kind != probe.DeclType {
			t.Errorf("kind %v", q.Kind)
		}
		got = append(got, q.Name)
	}
	want := "FILE,Old,Rect,_IO_FILE,geo::Body,geo::Vec,std::pair<int, Old>,std::vector<geo::Body>,std::vector<long>"
	// A name is asked once; the library's internals and the typedefs of C (int64_t) are not.
	if strings.Join(got, ",") != want {
		t.Errorf("needs\n got %s\nwant %s", strings.Join(got, ","), want)
	}
	// What is known or was refused is not asked again.
	m.Add(&probe.DeclTable{Canon: map[string]string{"geo::Vec": "geo::Vec", "Rect": "Rect"}, Errors: map[string]string{"type Old": "no"}})
	got = nil
	for _, q := range m.Needs(rs) {
		got = append(got, q.Name)
	}
	if strings.Contains(strings.Join(got, ","), "geo::Vec") || strings.Contains(strings.Join(got, ","), "Rect") || strings.Contains(strings.Join(got, ","), "Old,") {
		t.Errorf("asked again: %v", got)
	}
}

func TestNeedsMore(t *testing.T) {
	m := New(table(), LP64)
	var got []string
	for _, q := range m.NeedsMore() {
		got = append(got, q.Name)
	}
	// geo::Body.w is "Missing" in geo: asked as geo::Missing and Missing; the signature of
	// geo::f mentions Vec, which is known in geo. The private base geo::Priv is not asked.
	want := "Missing,geo::Missing"
	if strings.Join(got, ",") != want {
		t.Errorf("needs more: %v, want %s", got, want)
	}
	// The library's own records are not followed.
	dstd := &probe.DeclTable{Records: map[string]*probe.Record{"std::thing<int>": {Name: "std::thing<int>", Scope: "std", Tag: "struct", Complete: true,
		Fields: []probe.Field{{Name: "p", Type: tt("Whatever"), Access: probe.Public}}}}}
	if more := New(dstd, LP64).NeedsMore(); len(more) != 0 {
		t.Errorf("a record of std: %v", more)
	}
	// A private member is not followed; a public one is, under every scope.
	d0 := &probe.DeclTable{Records: map[string]*probe.Record{"a::R": {Name: "a::R", Scope: "a", Tag: "struct", Complete: true,
		Fields: []probe.Field{{Name: "p", Type: tt("Hidden"), Access: probe.Private}, {Name: "q", Type: tt("Shown"), Access: probe.Public}, {Name: "r", Type: tt("Prot"), Access: probe.Protected}},
		Bases:  []probe.Base{{Type: tt("PrivBase"), Access: probe.Private}, {Type: tt("PubBase"), Access: probe.Public}}}}}
	got = nil
	for _, q := range New(d0, LP64).NeedsMore() {
		got = append(got, q.Name)
	}
	if strings.Join(got, ",") != "PubBase,Shown,a::PubBase,a::Shown" {
		t.Errorf("access: %v", got)
	}
	// A template argument is asked for before the template: geo::Wrap<Vec> waits for Vec.
	d := &probe.DeclTable{Records: map[string]*probe.Record{"geo::Holder": {Name: "geo::Holder", Scope: "geo", Tag: "struct", Complete: true,
		Fields: []probe.Field{{Name: "w", Type: tt("Wrap<Later>"), Access: probe.Public}, {Name: "s", Type: tt("std::string"), Access: probe.Public},
			{Name: "i", Type: tt("_Internal"), Access: probe.Public}}}}}
	m2 := New(d, LP64)
	got = nil
	for _, q := range m2.NeedsMore() {
		got = append(got, q.Name)
	}
	if strings.Join(got, ",") != "Later,geo::Later" {
		t.Errorf("first round: %v", got)
	}
	m2.Add(&probe.DeclTable{Canon: map[string]string{"geo::Later": "geo::Later"}})
	got = nil
	for _, q := range m2.NeedsMore() {
		got = append(got, q.Name)
	}
	if strings.Join(got, ",") != "Wrap<geo::Later>,geo::Wrap<geo::Later>" {
		t.Errorf("second round: %v", got)
	}
	// Once the refusals are in, nothing more is asked.
	m.Add(&probe.DeclTable{Errors: map[string]string{"type Missing": "x", "type geo::Missing": "x"}})
	if more := m.NeedsMore(); len(more) != 0 {
		t.Errorf("still asking: %v", more)
	}
}

func TestIsInternal(t *testing.T) {
	for name, want := range map[string]bool{
		"std::vector<long>": false, "std::_Vector_base<long>": true, "__gnu_cxx::__normal_iterator<long *>": true,
		"geo::Vec": false, "_IO_FILE": false, "_Foo": false, "std::_Foo": true, "_foo": false, "std::map<K, std::_Rb_tree<K>>": true, "a::__b": true,
	} {
		if got := isInternal(name); got != want {
			t.Errorf("%q: %v", name, got)
		}
	}
}
