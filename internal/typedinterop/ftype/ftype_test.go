package ftype

import (
	"go/types"
	"math/big"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func tt(s string) probe.TypeText {
	t := ctype.MustParse(s)
	return probe.TypeText{Text: s, T: t}
}

// table is a small hand-made declaration table: geo::Vec, geo::Body (with a
// field of Vec relative to geo), a self-referential Node, an opaque handle,
// a derived class, enums, and typedefs.
func table() *probe.DeclTable {
	d := &probe.DeclTable{
		Records: map[string]*probe.Record{}, Enums: map[string]*probe.Enum{}, Funcs: map[string]*probe.FuncSet{},
		Canon: map[string]string{}, Errors: map[string]string{},
	}
	d.Records["geo::Vec"] = &probe.Record{Name: "geo::Vec", Scope: "geo", Tag: "struct", Complete: true, Size: 16, Align: 8,
		Fields: []probe.Field{{Name: "x", Type: tt("double"), Access: probe.Public}, {Name: "y", Type: tt("double"), Access: probe.Public},
			{Name: "flags", Type: tt("unsigned int"), Access: probe.Public, Bits: 3}, {Name: "secret", Type: tt("int"), Access: probe.Private}},
		Statics: []probe.Static{{Name: "count", Type: tt("int"), Access: probe.Public}},
		Methods: []probe.Method{
			{Name: "len", Kind: probe.KindMethod, Sig: tt("double () const"), Access: probe.Public},
			{Name: "operator+", Kind: probe.KindOperator, Sig: tt("Vec (const Vec &) const"), Access: probe.Public},
			{Name: "scale", Kind: probe.KindMethod, Sig: tt("void (double)"), Access: probe.Public},
			{Name: "scale", Kind: probe.KindMethod, Sig: tt("void (double, double)"), Access: probe.Public},
			{Name: "Vec", Kind: probe.KindCtor, Sig: tt("void (double, double)"), Access: probe.Public},
			{Name: "hidden", Kind: probe.KindMethod, Sig: tt("int ()"), Access: probe.Private},
		},
	}
	d.Records["geo::Body"] = &probe.Record{Name: "geo::Body", Scope: "geo", Tag: "struct", Complete: true, Size: 40, Align: 8,
		Fields: []probe.Field{{Name: "pos", Type: tt("Vec"), Access: probe.Public}, {Name: "next", Type: tt("Body *"), Access: probe.Public},
			{Name: "name", Type: tt("const char *"), Access: probe.Public}, {Name: "arr", Type: tt("int[4]"), Access: probe.Public},
			{Name: "cb", Type: tt("void (*)(int, Vec)"), Access: probe.Public}, {Name: "w", Type: tt("Missing"), Access: probe.Public}},
		Methods: []probe.Method{{Name: "len", Kind: probe.KindMethod, Sig: tt("int ()"), Access: probe.Public}},
		Bases:   []probe.Base{{Type: tt("geo::Vec"), Access: probe.Public}, {Type: tt("geo::Priv"), Access: probe.Private}},
	}
	d.Records["geo::Der"] = &probe.Record{Name: "geo::Der", Scope: "geo", Tag: "struct", Complete: true, Size: 24, Align: 8,
		Fields: []probe.Field{{Name: "x", Type: tt("int"), Access: probe.Public}}, Bases: []probe.Base{{Type: tt("geo::Vec"), Access: probe.Public}}}
	d.Canon["geo::Der"] = "geo::Der"
	d.Records["geo::ProtD"] = &probe.Record{Name: "geo::ProtD", Scope: "geo", Tag: "class", Complete: true,
		Bases: []probe.Base{{Type: tt("geo::Vec"), Access: probe.Protected}}}
	d.Records["geo::PrivD"] = &probe.Record{Name: "geo::PrivD", Scope: "geo", Tag: "class", Complete: true,
		Bases: []probe.Base{{Type: tt("geo::Vec"), Access: probe.Private}}}
	d.Canon["geo::ProtD"], d.Canon["geo::PrivD"] = "geo::ProtD", "geo::PrivD"
	d.Records["Opaque"] = &probe.Record{Name: "Opaque", Tag: "struct", Complete: false, Size: -1, Align: -1}
	d.Records["_IO_FILE"] = &probe.Record{Name: "_IO_FILE", Tag: "struct", Complete: false, Size: -1, Align: -1}
	d.Canon["FILE"] = "_IO_FILE"
	d.Records["Vec"] = &probe.Record{Name: "Vec", Tag: "struct", Complete: true, Size: 4, Align: 4, Fields: []probe.Field{{Name: "global", Type: tt("int"), Access: probe.Public}}}
	d.Canon["Vec"] = "Vec"
	d.Canon["Id"] = "unsigned int"
	d.Canon["geo::Vec"] = "geo::Vec"
	d.Canon["geo::Body"] = "geo::Body"
	d.Canon["geo::Priv"] = "geo::Priv"
	d.Enums["geo::Kind"] = &probe.Enum{Name: "geo::Kind", Scope: "geo", Scoped: true, Underlying: tt("unsigned char"), Size: 1,
		Values: []probe.Enumerator{{Name: "A", Value: big.NewInt(0)}, {Name: "B", Value: big.NewInt(5)}}}
	d.Enums["Old"] = &probe.Enum{Name: "Old", Underlying: tt("int"), Size: 4,
		Values: []probe.Enumerator{{Name: "O1", Value: big.NewInt(-1)}, {Name: "O2", Value: big.NewInt(0)}}}
	d.Funcs["geo::f"] = &probe.FuncSet{Name: "geo::f", Scope: "geo", Overloads: []probe.Overload{
		{Kind: "function", Sig: tt("Vec (const Vec &, int)")}, {Kind: "function", Sig: tt("long (long, const char *, ...)")}}}
	return d
}

func show(m *Mapper, text, written string) string {
	var w *ctype.Type
	if written != "" {
		w = ctype.MustParse(written)
	}
	v := m.Map(ctype.MustParse(text), w)
	return describe(m, v)
}

func describe(m *Mapper, v Mapped) string {
	if v.Kind == Opaque {
		return "opaque: " + v.Reason
	}
	t := "void"
	if v.Type != nil {
		t = m.TypeString(v.Type)
	}
	s := v.Kind.String() + " " + t
	if v.Alias != "" {
		s += " as " + v.Alias
	}
	if v.Ref != "" {
		s += " " + v.Ref
	}
	if v.Incomplete {
		s += " incomplete"
	}
	return s
}

func TestMapTypes(t *testing.T) {
	m := New(table(), LP64)
	for _, c := range []struct{ in, written, want string }{
		{"int", "", "scalar C.int"},
		{"unsigned long", "size_t", "scalar C.ulong as size_t"},
		{"long", "int64_t", "scalar C.long as int64_t"},
		{"unsigned char", "uint8_t", "scalar C.uchar as uint8_t"},
		{"unsigned char", "Byte", "scalar C.uchar"},
		{"char", "", "scalar C.char"},
		{"signed char", "", "scalar C.schar"},
		{"unsigned long long", "", "scalar C.ulonglong"},
		{"float", "", "scalar C.float"},
		{"double", "", "scalar C.double"},
		{"bool", "", "scalar C.bool"},
		{"wchar_t", "", "scalar C.wchar_t"},
		{"char16_t", "", "scalar C.char16_t"},
		{"void", "", "void void"},
		{"const int", "", "scalar C.int"},
		{"void *", "", "pointer unsafe.Pointer"},
		{"const char *", "", "pointer *C.char"},
		{"char **", "", "pointer **C.char"},
		{"int *const", "", "pointer *C.int"},
		{"int &", "", "scalar C.int &"},
		{"const long &&", "", "scalar C.long &&"},
		{"int[4]", "", "array [4]C.int"},
		{"int (&)[4]", "", "array [4]C.int &"},
		{"int[]", "", "array [0]C.int"},
		{"int (int, long)", "", "func func(C.int, C.long) C.int"},
		{"void (*)(int)", "", "funcptr func(C.int)"},
		{"int (*)(const char *, ...)", "", "funcptr func(*C.char) C.int"},
		{"geo::Vec", "", "record geo.Vec"},
		{"const geo::Vec &", "", "record geo.Vec &"},
		{"geo::Vec *", "", "pointer *geo.Vec"},
		{"geo::Kind", "", "enum geo.Kind"},
		{"Old", "", "enum Old"},
		{"Opaque", "", "record Opaque incomplete"},
		{"Opaque *", "", "pointer *Opaque"},
		{"_IO_FILE *", "FILE *", "pointer *_IO_FILE"},
		{"Unknown", "", "record Unknown incomplete"},
		{"std::vector<long>", "", "record std.vector[C.long] incomplete"},
		{"std::basic_string<char>", "", "record std.string incomplete"},
		{"std::map<std::basic_string<char>, std::vector<int>>", "", "record std.map[std.basic_string[C.char], std.vector[C.int]] incomplete"},
		{"gocpp_probe_internal_pr::string", "", "go string"},
		{"gocpp_probe_internal_pr::slice<long>", "", "go []C.long"},
		{"gocpp_probe_internal_pr::slice<gocpp_probe_internal_pr::string>", "", "go []string"},
		{"gocpp_probe_internal_pr::array<unsigned char, 32>", "", "go [32]C.uchar"},
		{"gocpp_probe_internal_pr::array<unsigned char, 32UL>", "", "go [32]C.uchar"},
		{"gocpp_probe_internal_pr::map<gocpp_probe_internal_pr::string, long>", "", "go map[string]C.long"},
		{"gocpp_probe_internal_pr::map<geo::Vec, long>", "", "go map[geo.Vec]C.long"},
		{"gocpp_probe_internal_pr::map<gocpp_probe_internal_pr::slice<long>, long>", "", "opaque: a stand-in gocpp_probe_internal_pr::map<gocpp_probe_internal_pr::slice<long>, long> whose parts have no gocpp view"},
		{"gocpp_probe_internal_pr::slice<void>", "", "opaque: a stand-in gocpp_probe_internal_pr::slice<void> whose parts have no gocpp view"},
		{"std::nullptr_t", "", "nullptr C.nullptr_t"},
		{"__int128", "", "opaque: the type __int128 has no gocpp view"},
		{"int S::*", "", "opaque: pointer to member"},
		{"void (S::*)()", "", "opaque: pointer to member"},
		{"(unnamed struct)", "", "opaque: a type with no name (unnamed struct)"},
		{"(unnamed struct) *", "", "opaque: pointer to a type with no name (unnamed struct)"},
		{"<overloaded function type>", "", "opaque: <overloaded function type>"},
		{"type-parameter-0-0", "", "opaque: a dependent type"},
		{"typename T::type", "", "opaque: a dependent type"},
		{"FILE", "", "record _IO_FILE incomplete"}, // a typedef name that was asked for: canonical _IO_FILE
		{"Id", "", "scalar C.uint as Id"},
	} {
		if got := show(m, c.in, c.written); got != c.want {
			t.Errorf("%q (written %q): got %q, want %q", c.in, c.written, got, c.want)
		}
	}
}

// The named types have the Go basic types of the same size as underlying types.
func TestUnderlying(t *testing.T) {
	m := New(nil, LP64)
	for c, want := range map[string]types.BasicKind{
		"char": types.Int8, "signed char": types.Int8, "unsigned char": types.Uint8, "short": types.Int16, "unsigned short": types.Uint16,
		"int": types.Int32, "unsigned int": types.Uint32, "long": types.Int64, "unsigned long": types.Uint64,
		"long long": types.Int64, "unsigned long long": types.Uint64, "float": types.Float32, "double": types.Float64, "long double": types.Float64,
		"bool": types.Bool, "wchar_t": types.Int32, "char8_t": types.Uint8, "char16_t": types.Uint16, "char32_t": types.Uint32,
	} {
		v := m.Map(ctype.MustParse(c), nil)
		if b, ok := v.Type.Underlying().(*types.Basic); !ok || b.Kind() != want {
			t.Errorf("%s: underlying %v, want kind %v", c, v.Type.Underlying(), want)
		}
	}
	// Two C types with the same Go representation are different types (C.int is not C.uint).
	if m.Map(ctype.MustParse("int"), nil).Type == m.Map(ctype.MustParse("unsigned int"), nil).Type {
		t.Error("int and unsigned int are one type")
	}
	if m.Map(ctype.MustParse("long"), nil).Type == m.Map(ctype.MustParse("long long"), nil).Type {
		t.Error("long and long long are one type")
	}
	if !types.Identical(m.Map(ctype.MustParse("const int"), nil).Type, m.Map(ctype.MustParse("int"), nil).Type) {
		t.Error("const int is not int")
	}
}

func TestFlexibleArray(t *testing.T) {
	m := New(nil, LP64)
	if v := m.Map(ctype.MustParse("char[]"), nil); !v.Flexible || v.Kind != Array {
		t.Errorf("char[]: %+v", v)
	}
	if v := m.Map(ctype.MustParse("char[0]"), nil); v.Flexible {
		t.Errorf("char[0] is flexible")
	}
	if v := m.Map(ctype.MustParse("char[4]"), nil); v.Flexible || v.Elem == nil || v.Elem.Kind != Scalar {
		t.Errorf("char[4]: %+v", v)
	}
}

func TestSignatureDetails(t *testing.T) {
	m := New(table(), LP64)
	v := m.Map(ctype.MustParse("long (const geo::Vec &, int *, double &&, ...) const noexcept"), nil)
	if v.Kind != Func || v.Partial {
		t.Fatalf("%+v", v)
	}
	sig := v.Type.(*types.Signature)
	if got := m.TypeString(sig); got != "func(geo.Vec, *C.int, C.double) C.long" {
		t.Errorf("signature %s", got)
	}
	info := m.Sig(sig)
	if info == nil || !info.Variadic || !info.ConstQ || !info.Noexcept || len(info.Params) != 3 {
		t.Fatalf("%+v", info)
	}
	if info.Params[0].Ref != "&" || !info.Params[0].Const || info.Params[1].Ref != "" || info.Params[2].Ref != "&&" {
		t.Errorf("params %+v", info.Params)
	}
	if info.Result.Kind != Scalar {
		t.Errorf("result %+v", info.Result)
	}
	// An opaque parameter makes the function partial, not opaque.
	p := m.Map(ctype.MustParse("void (int S::*, int)"), nil)
	if p.Kind != Func || !p.Partial {
		t.Errorf("partial: %+v", p)
	}
	// No result.
	if r := m.Map(ctype.MustParse("void ()"), nil); r.Type.(*types.Signature).Results().Len() != 0 {
		t.Errorf("void result: %v", r.Type)
	}
	if m.Sig(types.NewSignatureType(nil, nil, nil, nil, nil, false)) != nil {
		t.Error("Sig of a foreign signature")
	}
}

func TestRecordFields(t *testing.T) {
	m := New(table(), LP64)
	v := m.Map(ctype.MustParse("geo::Body"), nil)
	n, ok := v.Type.(*types.Named)
	if !ok || v.Incomplete {
		t.Fatalf("%+v", v)
	}
	st := n.Underlying().(*types.Struct)
	var got []string
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		got = append(got, f.Name()+":"+m.TypeString(f.Type()))
		if f.Embedded() {
			got[len(got)-1] += "(embedded)"
		}
	}
	// pos resolves Vec in namespace geo; next is a pointer to the record itself; the
	// private base is not a field; Missing is not known: its field has an invalid type.
	want := "pos:geo.Vec next:*geo.Body name:*C.char arr:[4]C.int cb:func(C.int, geo.Vec) w:invalid type Vec:geo.Vec(embedded)"
	if strings.Join(got, " ") != want {
		t.Errorf("fields\n got %s\nwant %s", strings.Join(got, " "), want)
	}
	// The self reference is the same type.
	if st.Field(1).Type().(*types.Pointer).Elem() != types.Type(n) {
		t.Error("next does not point to Body")
	}
	// Promotion through the embedded base works with go/types.
	obj, _, _ := types.LookupFieldOrMethod(n, true, m.Pkg, "x")
	if obj == nil {
		t.Error("x is not promoted from Vec")
	}
	if obj, _, _ := types.LookupFieldOrMethod(n, true, m.Pkg, "secret"); obj != nil {
		t.Error("a private field is visible")
	}
	// Bit-field tag.
	vec := m.Map(ctype.MustParse("geo::Vec"), nil).Type.(*types.Named).Underlying().(*types.Struct)
	if vec.NumFields() != 3 || !strings.Contains(vec.Tag(2), "bits=3") {
		t.Errorf("Vec: %d fields, tag %q", vec.NumFields(), vec.Tag(2))
	}
	// The same type is the same object.
	if m.Map(ctype.MustParse("geo::Vec"), nil).Type != m.Map(ctype.MustParse("const geo::Vec &"), nil).Type {
		t.Error("two mappings of geo::Vec differ")
	}
}

func TestMembers(t *testing.T) {
	m := New(table(), LP64)
	vec := m.Map(ctype.MustParse("geo::Vec"), nil).Type
	ms := m.MembersOf(vec)
	if ms == nil {
		t.Fatal("no members")
	}
	if f := ms.Lookup("x"); f == nil || f.Kind != "field" || f.Access != probe.Public {
		t.Errorf("x: %+v", f)
	}
	if f := ms.Lookup("secret"); f == nil || f.Access != probe.Private {
		t.Errorf("secret: %+v", f)
	}
	sc := ms.Lookup("scale")
	if sc == nil || sc.Kind != "method" || len(sc.Methods) != 2 {
		t.Fatalf("scale: %+v", sc)
	}
	if got := m.TypeString(sc.Methods[1].Sig.Type); got != "func(C.double, C.double)" {
		t.Errorf("scale overload: %s", got)
	}
	ln := ms.Lookup("len")
	if ln == nil || !ln.Methods[0].Const || m.TypeString(ln.Methods[0].Sig.Type) != "func() C.double" {
		t.Errorf("len: %+v", ln)
	}
	plus := ms.Lookup("operator+")
	if plus == nil || m.TypeString(plus.Methods[0].Sig.Type) != "func(geo.Vec) geo.Vec" {
		t.Errorf("operator+: %+v", plus)
	}
	if ms.Lookup("Vec") != nil {
		t.Error("a constructor is a member")
	}
	if st := ms.Lookup("count"); st == nil || st.Kind != "static" {
		t.Errorf("count: %+v", st)
	}
	if len(ms.Statics) != 1 || ms.Statics[0].Name != "count" || m.TypeString(ms.Statics[0].Mapped.Type) != "C.int" {
		t.Errorf("statics %+v", ms.Statics)
	}
	if len(ms.Methods) != 6 || ms.Lookup("nope") != nil {
		t.Errorf("methods %d", len(ms.Methods))
	}
	// The derived class sees Vec's members through its public base.
	body := m.MembersOf(m.Map(ctype.MustParse("geo::Body"), nil).Type)
	if f := body.Lookup("scale"); f == nil || f.Of != "geo::Vec" || len(f.Methods) != 2 {
		t.Errorf("Body.scale: %+v", f)
	}
	// Body's own len hides Vec's.
	if f := body.Lookup("len"); f == nil || f.Of != "geo::Body" || len(f.Methods) != 1 || m.TypeString(f.Methods[0].Sig.Type) != "func() C.int" {
		t.Errorf("Body.len: %+v", f)
	}
	if f := body.Lookup("pos"); f == nil || f.Of != "geo::Body" {
		t.Errorf("Body.pos: %+v", f)
	}
	// A field of the derived class hides the base's field of the same name.
	der := m.MembersOf(m.Map(ctype.MustParse("geo::Der"), nil).Type)
	if f := der.Lookup("x"); f == nil || f.Of != "geo::Der" || m.TypeString(f.Type.Type) != "C.int" {
		t.Errorf("Der.x: %+v", f)
	}
	if f := der.Lookup("y"); f == nil || f.Of != "geo::Vec" {
		t.Errorf("Der.y: %+v", f)
	}
	// The members of a protected base are at most protected, those of a private base private.
	for name, want := range map[string]probe.Access{"geo::ProtD": probe.Protected, "geo::PrivD": probe.Private} {
		ms := m.MembersOf(m.Map(ctype.MustParse(name), nil).Type)
		if x := ms.Lookup("x"); x == nil || x.Access != want {
			t.Errorf("%s.x: %+v, want %s", name, x, want)
		}
		if s := ms.Lookup("secret"); s == nil || s.Access != probe.Private {
			t.Errorf("%s.secret: %+v", name, s)
		}
	}
	// A private base's members are private.
	// An incomplete record has no members.
	if m.MembersOf(m.Map(ctype.MustParse("Opaque"), nil).Type) != nil {
		t.Error("members of an incomplete record")
	}
	if m.MembersOf(types.Typ[types.Int]) != nil {
		t.Error("members of int")
	}
}

func TestEnums(t *testing.T) {
	m := New(table(), LP64)
	k := m.Map(ctype.MustParse("geo::Kind"), nil)
	if !k.Scoped || k.Kind != Enum {
		t.Errorf("%+v", k)
	}
	if u := k.Type.Underlying(); u != types.Typ[types.Uint8] {
		t.Errorf("underlying %v", u)
	}
	cs := m.Enumerators(k.Type)
	if len(cs) != 2 || cs[1].Name() != "B" || cs[1].Val().String() != "5" || cs[1].Type() != k.Type {
		t.Errorf("enumerators %v", cs)
	}
	old := m.Map(ctype.MustParse("Old"), nil)
	if old.Scoped || old.Type.Underlying() != types.Typ[types.Int32] {
		t.Errorf("Old %+v", old)
	}
	if cs := m.Enumerators(old.Type); len(cs) != 2 || cs[0].Val().String() != "-1" {
		t.Errorf("Old enumerators %v", cs)
	}
	if m.Enumerators(types.Typ[types.Int]) != nil {
		t.Error("enumerators of int")
	}
	if m.Canonical(k.Type) != "geo::Kind" || m.Scope(k.Type) != "geo" {
		t.Errorf("canonical %q scope %q", m.Canonical(k.Type), m.Scope(k.Type))
	}
}

func TestTargets(t *testing.T) {
	win := New(nil, LLP64)
	if u := win.Map(ctype.MustParse("long"), nil).Type.Underlying(); u != types.Typ[types.Int32] {
		t.Errorf("long on Windows: %v", u)
	}
	if u := win.Map(ctype.MustParse("unsigned long"), nil).Type.Underlying(); u != types.Typ[types.Uint32] {
		t.Errorf("unsigned long on Windows: %v", u)
	}
	if u := win.Map(ctype.MustParse("wchar_t"), nil).Type.Underlying(); u != types.Typ[types.Uint16] {
		t.Errorf("wchar_t on Windows: %v", u)
	}
	if u := win.Map(ctype.MustParse("long long"), nil).Type.Underlying(); u != types.Typ[types.Int64] {
		t.Errorf("long long on Windows: %v", u)
	}
	lin := New(nil, Target{})
	if u := lin.Map(ctype.MustParse("long"), nil).Type.Underlying(); u != types.Typ[types.Int64] {
		t.Errorf("long on Linux: %v", u)
	}
	if u := lin.Map(ctype.MustParse("char"), nil).Type.Underlying(); u != types.Typ[types.Int8] {
		t.Errorf("char: %v", u)
	}
	uc := New(nil, Target{LongBits: 64, CharUnsigned: true})
	if u := uc.Map(ctype.MustParse("char"), nil).Type.Underlying(); u != types.Typ[types.Uint8] {
		t.Errorf("unsigned char target: %v", u)
	}
	if u := uc.Map(ctype.MustParse("signed char"), nil).Type.Underlying(); u != types.Typ[types.Int8] {
		t.Errorf("signed char on an unsigned char target: %v", u)
	}
}

// The typedef names of the C library follow the target: size_t is as wide as a pointer, int64_t
// is 64 bits (long on LP64, long long on LLP64, which has a 32-bit long).
func TestTypedefsFollowTheTarget(t *testing.T) {
	for _, c := range []struct {
		target Target
		name   string
		want   map[string]types.BasicKind
	}{
		{LP64, "LP64", map[string]types.BasicKind{
			"size_t": types.Uint64, "uintptr_t": types.Uint64, "ssize_t": types.Int64, "ptrdiff_t": types.Int64, "intptr_t": types.Int64,
			"int64_t": types.Int64, "uint64_t": types.Uint64, "int32_t": types.Int32, "uint32_t": types.Uint32, "int16_t": types.Int16,
			"uint16_t": types.Uint16, "int8_t": types.Int8, "uint8_t": types.Uint8, "std::size_t": types.Uint64, "std::int64_t": types.Int64}},
		{LLP64, "LLP64", map[string]types.BasicKind{
			"size_t": types.Uint64, "uintptr_t": types.Uint64, "ssize_t": types.Int64, "ptrdiff_t": types.Int64, "intptr_t": types.Int64,
			"int64_t": types.Int64, "uint64_t": types.Uint64, "int32_t": types.Int32, "uint32_t": types.Uint32, "std::size_t": types.Uint64, "std::uint64_t": types.Uint64}},
		{ILP32, "ILP32", map[string]types.BasicKind{
			"size_t": types.Uint32, "uintptr_t": types.Uint32, "ssize_t": types.Int32, "ptrdiff_t": types.Int32, "intptr_t": types.Int32,
			"int64_t": types.Int64, "uint64_t": types.Uint64, "int32_t": types.Int32, "std::size_t": types.Uint32}},
	} {
		m := New(nil, c.target)
		for name, kind := range c.want {
			v := m.Map(ctype.MustParse(name), nil)
			if b, ok := v.Type.Underlying().(*types.Basic); v.Kind != Scalar || !ok || b.Kind() != kind || v.Alias != name {
				t.Errorf("%s: %s is %s (alias %q), want kind %v", c.name, name, describe(m, v), v.Alias, kind)
			}
		}
	}
	// With the canonical type and the written one, the alias is kept only when they agree.
	win := New(nil, LLP64)
	if v := win.Map(ctype.MustParse("unsigned long long"), ctype.MustParse("size_t")); v.Alias != "size_t" || v.Type.Underlying() != types.Typ[types.Uint64] {
		t.Errorf("size_t on Windows: %+v", v)
	}
	if v := win.Map(ctype.MustParse("unsigned long"), ctype.MustParse("size_t")); v.Alias != "" {
		t.Errorf("a size_t that is unsigned long on Windows: %+v", v)
	}
	// An unqualified Named in a signature takes the same path.
	if v := win.MapIn(ctype.MustParse("size_t (const char *)"), "ns"); v.Kind != Func || win.TypeString(v.Type) != "func(*C.char) C.ulonglong" {
		t.Errorf("signature on Windows: %s", win.TypeString(v.Type))
	}
}

func TestRelativeNames(t *testing.T) {
	m := New(table(), LP64)
	for _, c := range []struct{ in, scope, want string }{
		{"Vec", "geo", "record geo.Vec"},
		{"Vec *", "geo", "pointer *geo.Vec"},
		{"geo::Vec", "geo", "record geo.Vec"},
		{"Vec", "geo::inner", "record geo.Vec"}, // an enclosing namespace
		{"Old", "geo", "enum Old"},              // the global one
		{"Old", "", "enum Old"},
		{"Vec", "", "record Vec"},        // a canonical name: no scope search, the global one
		{"Vec", "other", "record Vec"},   // not in geo: the global one
		{"Vec", "geo", "record geo.Vec"}, // in geo: the namespace's own shadows the global one
		{"Nowhere", "geo", "opaque: the name Nowhere is not resolved in geo"},
		{"Id", "geo", "scalar C.uint as Id"},
		{"std::vector<Vec>", "geo", "record std.vector[geo.Vec] incomplete"},
		{"std::string", "geo", "record std.string incomplete"},
		{"std::map<std::string, Vec *>", "geo", "record std.map[std.basic_string[C.char], *geo.Vec] incomplete"},
		{"std::vector<Nowhere>", "geo", "opaque: the name std::vector<Nowhere> is not resolved in geo"},
		{"Vec (const Vec &, Old)", "geo", "func func(geo.Vec, Old) geo.Vec"},
		{"unsigned long", "geo", "scalar C.ulong"},
		{"size_t", "geo", "scalar C.ulong as size_t"},
	} {
		got := describe(m, m.MapIn(ctype.MustParse(c.in), c.scope))
		if got != c.want {
			t.Errorf("%q in %q: got %q, want %q", c.in, c.scope, got, c.want)
		}
	}
}

func TestQualify(t *testing.T) {
	m := New(table(), LP64)
	for _, c := range []struct{ in, scope, want string }{
		{"Vec", "geo", "geo::Vec"}, {"Vec", "geo::x::y", "geo::Vec"}, {"Old", "geo", "Old"}, {"geo::Vec", "geo", "geo::Vec"},
		{"std::string", "geo", "std::basic_string<char>"}, {"std::vector<Vec>", "geo", "std::vector<geo::Vec>"},
		{"const Vec *", "geo", "const geo::Vec *"}, {"Vec &&", "geo", "geo::Vec &&"}, {"Vec[3]", "geo", "geo::Vec[3]"},
		{"void (*)(Vec, Old)", "geo", "void (*)(geo::Vec, Old)"}, {"int Vec::*", "geo", "int geo::Vec::*"},
		{"::Old", "geo", "Old"}, {"Nope", "geo", ""}, {"std::vector<Nope>", "geo", ""}, {"Vec", "", "Vec"},
	} {
		q, ok := m.qualify(ctype.MustParse(c.in), c.scope)
		got := ""
		if ok {
			got = q.String()
		}
		if got != c.want {
			t.Errorf("%q in %q: got %q, want %q", c.in, c.scope, got, c.want)
		}
	}
}

func TestRecursionAndDepth(t *testing.T) {
	d := table()
	// A cycle of bases and fields must end.
	d.Records["A"] = &probe.Record{Name: "A", Tag: "struct", Complete: true, Fields: []probe.Field{{Name: "b", Type: tt("B *"), Access: probe.Public}},
		Bases: []probe.Base{{Type: tt("B"), Access: probe.Public}}}
	d.Records["B"] = &probe.Record{Name: "B", Tag: "struct", Complete: true, Fields: []probe.Field{{Name: "a", Type: tt("A"), Access: probe.Public}},
		Bases: []probe.Base{{Type: tt("A"), Access: probe.Public}}}
	d.Canon["A"], d.Canon["B"] = "A", "B"
	m := New(d, LP64)
	v := m.Map(ctype.MustParse("A"), nil)
	if v.Kind != Record {
		t.Fatalf("%+v", v)
	}
	if ms := m.MembersOf(v.Type); ms == nil {
		t.Error("no members for the cycle")
	}
	deep := strings.Repeat("int *", 1)
	_ = deep
	var b strings.Builder
	b.WriteString("int")
	for i := 0; i < 100; i++ {
		b.WriteString(" *")
	}
	if got := show(m, b.String(), ""); !strings.HasPrefix(got, "opaque: pointer to") {
		t.Errorf("a hundred pointers: %.60s", got)
	}
}
