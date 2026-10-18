from lib import case
from cats3 import NORM

VERBS_STR = ["%v", "%s", "%d", "%x", "%X", "%q", "%+v", "%#v", "%10s", "%-10v|", "%.2s", "%t", "%p"]

def verbs_body(exprs, verbs=VERBS_STR):
    out = ""
    for e in exprs:
        out += '\tfor _, f := range []string{%s} {\n\t\tfmt.Printf("%%-6s[", f)\n\t\tfmt.Printf(f, %s)\n\t\tfmt.Printf("]\\n")\n\t}\n' % (", ".join('"%s"' % v for v in verbs), e)
    return out

# --- Stringer / error / GoStringer / Formatter ---
case("fmt_methods_stringer",
     "A value with a String method prints through it for %v %s %q %x %X and by Print and Println, not for %d or %#v; value and pointer receivers; the method set decides.",
     '''type V int

func (v V) String() string { return fmt.Sprintf("V<%d>", int(v)) }

type P struct{ n int }

func (p *P) String() string { return fmt.Sprintf("P<%d>", p.n) }

type S struct{ a, b int }

func (s S) String() string { return fmt.Sprintf("S<%d,%d>", s.a, s.b) }

type Sl []int

func (s Sl) String() string { return fmt.Sprintf("Sl%d", len(s)) }

type M map[string]int

func (m M) String() string { return "M" }

type F func()

func (f F) String() string { return "F" }

type Str string

func (s Str) String() string { return "Str:" + string(s) }

type Fl float64

func (f Fl) String() string { return fmt.Sprintf("Fl:%.1f", float64(f)) }

type Wrong struct{}

func (Wrong) String(x int) string { return "wrong" }

type Wrong2 struct{}

func (Wrong2) String() int { return 1 }

type Field struct{ String string }

type FuncField struct{ String func() string }
''',
     verbs_body(["V(3)", "S{1, 2}", "Sl{1, 2}", "M{}", "Str(\"q\")", "Fl(2.5)", "&S{3, 4}", "P{5}", "&P{6}"], ["%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"]) + '''	fmt.Println(V(1), S{2, 3}, &S{4, 5}, P{6}, &P{7}, Sl{1}, M{}, F(nil), Str("a"), Fl(1))
	fmt.Print(V(1), V(2), "x", V(3), "\\n")
	fmt.Println(Wrong{}, Wrong2{}, Field{"f"}, FuncField{}.String == nil)
	var sp *P
	var vp *S
	fmt.Println(sp, vp)
	fmt.Printf("%v %s %d\\n", sp, vp, sp)
	fmt.Printf("%v %v\\n", []V{1, 2}, map[V]V{1: 2})
	fmt.Printf("%v %v\\n", [1]S{{1, 2}}, struct{ X V }{9})
''')

case("fmt_methods_error",
     "A value with an Error method prints through it (error wins over String); %v %s %q %x %X, Print and Println; pointer and value receivers; nil errors.",
     '''type E1 struct{ code int }

func (e E1) Error() string { return fmt.Sprintf("E1:%d", e.code) }

type EP struct{ msg string }

func (e *EP) Error() string { return "EP:" + e.msg }

type Both struct{}

func (Both) Error() string  { return "the error" }
func (Both) String() string { return "the string" }

type Code int

func (c Code) Error() string { return fmt.Sprintf("code %d", int(c)) }

type Chain struct {
	msg  string
	next error
}

func (c *Chain) Error() string {
	if c.next == nil {
		return c.msg
	}
	return c.msg + ": " + c.next.Error()
}

func (c *Chain) Unwrap() error { return c.next }
''',
     verbs_body(["E1{7}", "&EP{\"m\"}", "Both{}", "Code(4)"], ["%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%10v|", "%-12s|", "%.3s"]) + '''	var err error = E1{1}
	fmt.Println(err, &EP{"x"}, Both{}, Code(2))
	var nilerr error
	fmt.Println(nilerr)
	fmt.Printf("%v %s %d %q|\\n", nilerr, nilerr, nilerr, nilerr)
	c := &Chain{"outer", &Chain{"inner", E1{3}}}
	fmt.Println(c)
	fmt.Printf("%v|%s|%q\\n", c, c, c)
	var ep *EP
	fmt.Println(ep == nil)
	fmt.Printf("%v\\n", []error{E1{1}, &EP{"p"}, nil, Code(3)})
	fmt.Printf("%v\\n", map[string]error{"a": E1{1}, "b": nil})
	fmt.Print(E1{1}, E1{2}, "s", Code(3), "\\n")
''')

case("fmt_methods_gostringer",
     "GoString is used for %#v only (and its result is printed as it is); Stringer is ignored by %#v; a type with both; inside composite values.",
     NORM + '''type G struct{ n int }

func (g G) GoString() string { return fmt.Sprintf("G(%d)", g.n) }

type GS struct{ n int }

func (g GS) GoString() string { return "GS!" }
func (g GS) String() string   { return "str" }

type PG struct{}

func (*PG) GoString() string { return "PG!" }

type Plain struct{ A int }

type Holder struct {
	G  G
	g  G
	GS GS
	P  *PG
}
''',
     '''	fmt.Printf("%#v %v %+v %s\\n", G{1}, G{1}, G{1}, G{1})
	fmt.Printf("%#v %v %s %#v\\n", GS{1}, GS{1}, GS{1}, &GS{2})
	fmt.Printf("%#v %#v %v\\n", &PG{}, PG{}, PG{})
	fmt.Printf("%#v\\n", []G{{1}, {2}})
	fmt.Printf("%#v\\n", map[string]G{"k": {3}})
	fmt.Printf("%#v\\n", [2]GS{})
	fmt.Println(norm(fmt.Sprintf("%#v", Holder{G{1}, G{2}, GS{3}, nil})))
	fmt.Printf("%#v %#v %#v\\n", Plain{1}, &Plain{2}, []Plain{{3}})
	fmt.Printf("%10v|%-10v|%#10v|%x\\n", G{1}, G{1}, G{1}, G{1})
	var a any = G{9}
	fmt.Printf("%#v %v\\n", a, a)
''' )

# Format
case("fmt_methods_formatter",
     "A Format(fmt.State, rune) method takes over every verb: it sees the flags, width and precision through the State, and can print with Fprintf, Write and Sprint; Formatter wins over Stringer and Error.",
     '''type F struct{ n int }

func (f F) Format(s fmt.State, verb rune) {
	w, wok := s.Width()
	p, pok := s.Precision()
	fmt.Fprintf(s, "F{verb=%c n=%d w=%d/%v p=%d/%v", verb, f.n, w, wok, p, pok)
	for _, c := range "+-# 0" {
		if s.Flag(int(c)) {
			fmt.Fprintf(s, " %c", c)
		}
	}
	fmt.Fprint(s, "}")
}

type G struct{}

func (G) Format(s fmt.State, verb rune) {
	s.Write([]byte("G!"))
	fmt.Fprintln(s, verb)
}

func (G) String() string { return "never" }

type H struct{}

func (H) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		fmt.Fprint(f, "hv")
	case 'd':
		fmt.Fprint(f, FormatIt(f, c))
	default:
		fmt.Fprintf(f, "%%!%c(H)", c)
	}
}

func FormatIt(f fmt.State, c rune) string { return fmt.FormatString(f, c) }

type PF struct{}

func (*PF) Format(s fmt.State, c rune) { fmt.Fprint(s, "PF") }
''',
     '''	for _, f := range []string{"%v", "%+v", "%#v", "%d", "%s", "%x", "%q", "%8.3d", "%-8v", "%+08.2f", "% x", "%#q", "%T", "%p", "%w"} {
		fmt.Printf("%s -> ", f)
		fmt.Printf(f, F{1})
		fmt.Printf("\\n")
	}
	fmt.Printf("%v|%d|%s\\n", G{}, G{}, G{})
	fmt.Println(F{2}, G{}, H{})
	fmt.Printf("%v %d %x %+10.3d %#-8x|\\n", H{}, H{}, H{}, H{}, H{})
	fmt.Printf("%v %v %v\\n", &PF{}, PF{}, []*PF{{}})
	fmt.Printf("%v\\n", []F{{1}, {2}})
	fmt.Printf("%v\\n", map[string]F{"a": {3}})
	fmt.Printf("%v\\n", struct{ X F }{F{4}})
	fmt.Printf("%v\\n", struct{ x F }{F{5}})
	fmt.Println(fmt.Sprint(F{6}), fmt.Sprintf("%5.1v", F{7}), fmt.Sprintln(F{8}) == "F{verb=v n=8 w=0/false p=0/false}\\n")
''')

# panics
case("fmt_methods_panic",
     "A panic in String, Error, GoString or Format is caught and printed as %!v(PANIC=String method: ...) with the panic value formatted by fmt; a nil pointer receiver prints <nil>; the value of a panic can be a string, an error, a float, a custom type.",
     '''type PS struct{}

func (PS) String() string { panic("boom") }

type PE struct{}

func (PE) Error() string { panic(fmt.Errorf("err %d", 7)) }

type PF struct{}

func (PF) String() string { panic(1.5) }

type PI struct{}

func (PI) String() string { panic(42) }

type PT struct{ a, b int }

func (PT) String() string { panic(struct{ a, b int }{1, 2}) }

type PN struct{}

func (PN) String() string {
	var m map[string]int
	m["a"] = 1
	return ""
}

type PX struct{}

func (PX) String() string {
	var a []int
	_ = a[3]
	return ""
}

type PG struct{}

func (PG) GoString() string { panic("gostring") }

type PFm struct{}

func (PFm) Format(f fmt.State, c rune) { panic("format") }

type NilRecv struct{ n int }

func (r *NilRecv) String() string { return fmt.Sprint(r.n) }

type ValRecv struct{ n int }

func (v ValRecv) String() string { return fmt.Sprint(v.n) }

type PD struct{}

func (PD) String() string {
	defer func() { recover() }()
	panic("recovered inside")
}

type Inner struct{ PS PS }
''',
     '''	fmt.Println(PS{}, PE{}, PF{}, PI{}, PT{}, PN{}, PX{})
	fmt.Printf("%v|%s|%q|%x|%d|%10v|%-10s|\\n", PS{}, PS{}, PS{}, PS{}, PS{}, PS{}, PS{})
	fmt.Printf("%#v|%+v|%v\\n", PG{}, PG{}, PG{})
	fmt.Printf("%v|%d|%s\\n", PFm{}, PFm{}, PFm{})
	fmt.Println([]PS{{}, {}}, map[string]PS{"k": {}}, Inner{}, &Inner{})
	var nr *NilRecv
	var vr *ValRecv
	fmt.Println(nr, vr)
	fmt.Printf("%v %s %d %+v|%10v|\\n", nr, nr, nr, vr, nr)
	fmt.Println([]*NilRecv{nr}, []*ValRecv{vr, {3}})
	fmt.Println(PD{})
	fmt.Println(fmt.Sprint(PS{}), len(fmt.Sprintln(PS{})))
''')

# the %T of a recovered assertion panic and friends
case("fmt_methods_recovered_values",
     "%T and %v of recovered panic values: a failed type assertion gives *runtime.TypeAssertionError, an index error runtime.boundsError, a nil map write runtime.plainError, panic(fmt.Errorf) gives *fmt.wrapError, custom values print as themselves.",
     '''type MyErr struct{ code int }

func (e MyErr) Error() string { return fmt.Sprint("myerr ", e.code) }

func try(f func()) (r any) {
	defer func() { r = recover() }()
	f()
	return
}
''',
     '''	var x any = "s"
	var m map[string]int
	var arr []int
	var ip *int
	zero := 0
	vals := []any{
		try(func() { _ = x.(int) }),
		try(func() { _ = x.(fmt.Stringer) }),
		try(func() { arr[5] = 1 }),
		try(func() { m["a"] = 1 }),
		try(func() { _ = *ip }),
		try(func() { _ = 1 / zero }),
		try(func() { panic("plain") }),
		try(func() { panic(MyErr{3}) }),
		try(func() { panic(fmt.Errorf("wrapped %d", 1)) }),
		try(func() { panic(fmt.Sprintf("%d", 12)) }),
		try(func() { panic(42) }),
		try(func() { panic(1.5) }),
		try(func() { var e error; panic(e) }),
	}
	for i, v := range vals {
		fmt.Printf("%d: %T | %v\\n", i, v, v)
	}
	fmt.Printf("%v\\n", try(func() {}))
''')

# --- interfaces / any ---
case("fmt_any_values",
     "Values of every kind inside an any: the dynamic type decides; %v %T %d %s on the same values; nil interfaces; typed nil pointers; named types.",
     '''type Celsius float64

type Pt struct{ X, Y int }

type Color int

func (c Color) String() string { return "color" + fmt.Sprint(int(c)) }
''',
     '''	var np *Pt
	var ns []int
	var nm map[string]int
	var nf func()
	var nerr error
	vals := []any{
		nil, true, 1, int8(-8), int16(16), int32(32), int64(64), uint(1), uint8(8), uint16(16), uint32(32), uint64(64), uintptr(9),
		1.5, float32(2.5), complex(1, 2), complex64(complex(3, 4)), "str", 'x', byte('y'),
		[]int{1, 2}, [2]string{"a", "b"}, map[string]int{"k": 1}, Pt{1, 2}, &Pt{3, 4},
		Celsius(21.5), Color(2), []Color{1, 2}, struct{ A int }{5}, np, ns, nm, nf == nil, nerr,
	}
	for i, v := range vals {
		fmt.Printf("%2d %-16T %v | %+v | %#v | %d | %s\\n", i, v, v, v, v, v, v)
	}
	fmt.Println(vals[0], vals[1], vals[len(vals)-3])
	fmt.Println(vals[:5], len(vals))
''')
tests_ptr_fix = True

case("fmt_any_nested",
     "Interfaces inside composite values: []any, map[string]any, struct fields of interface type, nested any, nil interfaces print <nil>; %#v shows the dynamic types.",
     '''type S struct {
	A any
	B error
	C fmt.Stringer
	D []any
	E map[string]any
}

type Str string

func (s Str) String() string { return "str:" + string(s) }
''',
     '''	s := S{1, nil, Str("c"), []any{1, "a", nil, []any{2}}, map[string]any{"k": []int{1}, "n": nil}}
	fmt.Printf("%v\\n%+v\\n%#v\\n", s, s, s)
	fmt.Println(S{})
	fmt.Printf("%v %+v %#v\\n", S{}, S{}, S{})
	var a any = s
	fmt.Printf("%v %T\\n", a, a)
	var b any = &s
	fmt.Println(fmt.Sprintf("%v", b) == fmt.Sprintf("&%v", s))
	var c any = []any{a}
	fmt.Println(c)
	var d any = map[any]any{1: "a", 2: []any{3}}
	fmt.Println(d)
	fmt.Printf("%v %v\\n", [2]any{}, [2]error{})
	fmt.Printf("%#v %#v\\n", [2]any{}, [2]error{})
	fmt.Printf("%d %s\\n", []any{1, "a", 2.5, nil}, []any{1, "a", 2.5, nil})
''')

case("fmt_any_typed_nil",
     "A typed nil pointer in an interface is not a nil interface: %v prints <nil> for it either way, %T tells them apart, a nil pointer with a value-receiver String method prints <nil>; %p of nil.",
     '''type T struct{ n int }

func (t *T) String() string {
	if t == nil {
		return "nil-T"
	}
	return "T"
}

type V struct{ n int }

func (v V) String() string { return "V" }

type E struct{}

func (e *E) Error() string { return "E" }
''',
     '''	var t *T
	var v *V
	var e *E
	var a any = t
	var b any = v
	var c any
	var err error = e
	fmt.Println(t, v, a, b, c, err)
	fmt.Printf("%T %T %T %T\\n", a, b, c, err)
	fmt.Println(a == nil, b == nil, c == nil, err == nil)
	fmt.Printf("%v %s %d %p %p\\n", a, b, c, t, c)
	fmt.Printf("%+v %#v %#v %#v\\n", a, a, b, c)
	var s fmt.Stringer = t
	fmt.Println(s, s == nil)
	var np *int
	fmt.Printf("%v %d %x %p %t\\n", np, np, np, np, np)
	fmt.Printf("%v %p %#v %v\\n", []*int{nil}, []int(nil), (*int)(nil), map[string]*int{"a": nil})
''')

case("fmt_type_names",
     "%T names the dynamic type as reflect does: predeclared and named types, composite types, funcs, channels-free interface types, pointers, anonymous structs and interfaces.",
     '''type Celsius float64

type Pt struct{ X int }

type List []Pt

type Dict map[string]Pt

type Fn func(int) string

type Arr [3]int

type Ptr *Pt

type Iface interface{ M() }

type Gen struct{}
''',
     '''	var i Iface
	var e error
	var st fmt.Stringer
	var ptrptr **Pt
	vals := []any{
		1, int8(1), uint(1), uintptr(1), 1.5, float32(1), complex(1, 1), complex64(1), "s", true, 'r', byte(1),
		Celsius(1), Pt{}, &Pt{}, List{}, Dict{}, Arr{}, Fn(nil), Ptr(nil), []int{}, []*Pt{}, [][]string{}, [2][3]bool{},
		map[string]int{}, map[Pt][]Celsius{}, map[any]error{}, func() {}, func(int, string) (bool, error) { return false, nil },
		func(...int) {}, struct{}{}, struct{ A int }{}, struct {
			A int `k:"v"`
			b []string
		}{}, []struct{ X, Y int }{}, ptrptr, i, e, st, &i, &e, []any{}, []error{}, [0]int{}, &[]int{}, &map[string]int{},
		new(int), new(Pt), new([]Pt), struct{ Pt }{}, struct {
			*Pt
			N int
		}{}, []interface{ M() }{}, []interface{ Error() string }{}, []fmt.Stringer{}, func(Iface) {},
		func(a, b int) (int, int) { return b, a },
	}
	n := 3
	var n64 int64 = 4
	var u uint = 5
	var u64 uint64 = 6
	var up uintptr = 7
	fmt.Printf("%T %T %T %T %T %T %T %T %T\\n", n, 1, n64, u, u64, up, 'r', byte(1), int32(2))
	fmt.Printf("%T %T %T %T %T\\n", 2.5, float32(1), "s", true, complex(1, 2))
	for _, v := range vals {
		fmt.Printf("%T\\n", v)
	}
	fmt.Printf("%T %T %T\\n", nil, vals, &vals)
''')
