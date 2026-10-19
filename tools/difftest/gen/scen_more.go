package gen

import (
	"fmt"
	"strconv"
	"strings"
)

// Scenarios of language corners that the first generator missed: complex numbers, the fmt
// methods (String, GoString, Format, Error and what fmt does when they panic), map keys that
// are interfaces, floats and NaNs (printed in the order fmt sorts them), float to integer
// conversions, invalid UTF-8 (overlong forms, surrogates, too large), the builtin print and
// println (their output goes to stderr, which the runner compares per program), and, behind a
// feature that is off by default, cap() after append growth.

func init() {
	register("complex", 7, []string{"complex"}, scenComplex)
	register("fmtmethods", 8, []string{"fmtmethods", "methods"}, scenFmtMethods)
	register("mapkeys", 7, []string{"mapkeys", "maps"}, scenMapKeys)
	register("floatconv", 8, []string{"floatconv", "floats"}, scenFloatConv)
	register("floatoor", 5, []string{"floatoor", "floats"}, scenFloatOOR)
	register("utf8bad", 7, []string{"utf8bad"}, scenUTF8Bad)
	register("println", 6, []string{"println"}, scenPrintln)
	register("capgrowth", 6, []string{"capgrowth", "generics"}, scenCapGrowth)
}

func scenComplex(g *Gen) {
	for i := g.between(1, 3); i > 0; i-- {
		g.tpl(`{
	var zero float64
	a, b := complex(@F1@, @F2@), complex(@F3@, @F4@)
	var z complex128
	fmt.Println(a, b, a+b, a-b, a*b, a/b, -a, a == b, a != b, real(a), imag(a), a/z, z/z)
	fmt.Printf("%v %.2f %g %e %+v %T %#v %8.1f|%-12v|\n", a, a, a, b, b, a, a, a, b)
	c64, d64 := complex64(a), complex64(b)
	fmt.Println(c64, c64*d64, c64/d64, c64+d64, real(c64), imag(c64), complex128(c64) == a)
	fmt.Printf("%v %T %.3g\n", c64*d64, c64, c64)
	const k = 3 + 4i
	fmt.Println(k, k*k, real(k), imag(k), a*k, k == 3+4i)
	cs := []complex128{a, b, 1, 2i, complex(0, 0)}
	sum := complex128(0)
	for _, c := range cs {
		sum += c * c
	}
	fmt.Println(cs, sum, len(map[complex128]int{a: 1, b: 2, a + 0: 3}))
	f := a
	f += b
	f *= b
	f /= complex(2, 0)
	f -= 1
	fmt.Println(f, complex(float32(1.5), 2.5), real(complex64(1+2i)))
	inf, nan := 1/zero, zero/zero
	ci := complex(inf, zero)
	fmt.Println(ci, ci*ci, a*ci, complex(nan, 1), a/ci, ci/a, ci/ci, complex(inf, inf)/complex(inf, inf), a/complex(zero, zero), complex(nan, nan) == complex(nan, nan))
	m := map[complex128]string{1 + 2i: "a", 1 + 1i: "b", 0: "c", 1 - 5i: "d"}
	fmt.Println(m)
}`, "F1", g.floatRaw(), "F2", g.floatRaw(), "F3", g.floatRaw(), "F4", g.floatRaw())
	}
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenFmtMethods(g *Gen) {
	names := map[string]string{}
	var kv []string
	for _, n := range []string{"TA", "TB", "TC", "TD", "TE", "TF", "TH", "TI", "TK", "TG"} {
		names[n] = g.topName(n)
		kv = append(kv, n, names[n])
	}
	types := `type @TA@ struct {
	X int
	Y string
}

func (a @TA@) String() string { return "A(" + fmt.Sprint(a.X) + ")" }

type @TB@ struct{ X int }

func (b *@TB@) String() string { return "B*" }

type @TC@ struct{ X int }

func (c @TC@) GoString() string { return "C!" }

type @TD@ struct{ X int }

func (d @TD@) Format(f fmt.State, verb rune) {
	w, wok := f.Width()
	p, pok := f.Precision()
	fmt.Fprintf(f, "D[%c w=%d,%v p=%d,%v +%v -%v #%v 0%v sp%v x=%d]", verb, w, wok, p, pok, f.Flag('+'), f.Flag('-'), f.Flag('#'), f.Flag('0'), f.Flag(' '), d.X)
}

type @TE@ struct{ X int }

func (e @TE@) Error() string  { return "E-error" }
func (e @TE@) String() string { return "E-string" }

type @TF@ int

func (f @TF@) String() string { return "F" + fmt.Sprint(int(f)) }

type @TH@ struct{}

func (@TH@) String() string { panic("boom") }

type @TI@ struct{ p *int }

func (i @TI@) String() string { return fmt.Sprint(*i.p) }

type @TK@ struct{ X, Y int }

func (k @TK@) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('#') {
			fmt.Fprint(f, "K#")
		} else {
			fmt.Fprintf(f, "K(%d,%d)", k.X, k.Y)
		}
	case 'd':
		fmt.Fprintf(f, "%d/%d", k.X, k.Y)
	default:
		fmt.Fprintf(f, "%%!%c(K)", verb)
	}
}

type @TG@ struct {
	A @TA@
	C []@TA@
	D map[string]@TA@
	e @TA@
	F fmt.Stringer
	G error
	H [2]@TF@
	K @TK@
}
`
	var pairs []string
	for i := 0; i < len(kv); i += 2 {
		pairs = append(pairs, "@"+kv[i]+"@", kv[i+1])
	}
	g.top.WriteString(strings.NewReplacer(pairs...).Replace(types) + "\n")
	g.tpl(`{
	a, b, c, d, e, f, h := @TA@{@N1@, @S1@}, @TB@{@N2@}, @TC@{@N3@}, @TD@{@N4@}, @TE@{@N5@}, @TF@(@N6@), @TH@{}
	pa, pb := &a, &b
	var nilA *@TA@
	k := @TK@{@N1@, @N2@}
	fmt.Println(a, pa, nilA, b, pb, c, d, e, f, h, @TI@{}, k)
	for _, v := range []any{a, pa, nilA, b, pb, c, d, e, f, h, @TI@{}, k, error(e)} {
		fmt.Printf("%v|%+v|%#v|%s|%q|%x|%d|%10v|%-8v|%5.1v|%t\n", v, v, v, v, v, v, v, v, v, v, v)
	}
	g := @TG@{a, []@TA@{a, {7, "x"}}, map[string]@TA@{"k": a}, a, a, error(nil), [2]@TF@{1, 2}, k}
	fmt.Printf("%v\n%+v\n%#v\n", g, g, g)
	g.F, g.G = f, e
	fmt.Printf("%v %s\n", g, g)
	fmt.Println([]@TB@{b}, []*@TB@{pb}, map[string]*@TB@{"k": pb}, [1]@TA@{a}, struct{ X @TB@ }{b}, struct{ X *@TB@ }{pb}, struct{ B *@TA@ }{pa})
	fmt.Printf("%v %+v %s %d %x\n", []@TF@{1, 2}, map[@TF@]@TF@{2: 3, 1: 4}, []@TF@{1}, []@TF@{1}, []@TF@{255})
	fmt.Println(fmt.Sprint(a, 1, "x", 2, 3, b, "y", c, d), fmt.Sprintln(a, b, 1, "z"))
	fmt.Print(a, b, "\n")
	fmt.Printf("%6.2v|%-6v|%06v|%+v|% v|%#v\n", a, f, @TF@(5), f, f, f)
	fmt.Printf("%v %v\n", &c, &d)
	fmt.Printf("%s %d %v\n", struct{ X, Y any }{a, f}, struct{ X, Y any }{a, f}, [2]error{e, nil})
	var st fmt.Stringer = f
	var gs fmt.GoStringer = c
	fmt.Println(st, gs.GoString(), st.String())
}`, append(kv, "N1", strconv.Itoa(g.between(-5, 99)), "N2", strconv.Itoa(g.between(0, 9)), "N3", strconv.Itoa(g.between(0, 9)),
		"N4", strconv.Itoa(g.between(0, 9)), "N5", strconv.Itoa(g.between(0, 9)), "N6", strconv.Itoa(g.between(-9, 99)), "S1", g.strRaw())...)
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenMapKeys(g *Gen) {
	g.tpl(`{
	var zero float64
	nan, negzero := zero/zero, -zero
	m := map[float64]int{1.5: 1, -0.5: 2, 0: 4}
	m[nan] = 3
	m[negzero] = 5
	m[1/zero] = 6
	m[-1/zero] = 7
	v1, ok1 := m[nan]
	delete(m, nan)
	fmt.Println(len(m), m, v1, ok1, m[0], m[negzero])
	clear(m)
	fmt.Println(len(m), m)
	mf := map[float32]string{}
	mf[float32(nan)] = "x"
	mf[0.25] = "q"
	fmt.Println(mf, len(mf))
	mi := map[any]int{nil: 9, 3: 1, 1: 2, 2: 3}
	fmt.Println(mi, mi[1], mi[int8(1)], mi[nil], len(mi))
	ms := map[any]string{"b": "1", "a": "2", "": "3"}
	mb := map[any]bool{true: true, false: false}
	mc := map[any]float64{1.5: 1, -2.5: 2, float64(0): 3}
	fmt.Println(ms, mb, mc)
	ma := map[[2]float64]int{{1, 2}: 1, {0, 5}: 2, {1, -1}: 3}
	ms2 := map[struct {
		A string
		B float64
	}]int{{"x", 2}: 1, {"x", 1}: 2, {"a", 9}: 3}
	fmt.Println(ma, ms2)
	var e1, e2 any = 1, 1.0
	fmt.Println(e1 == e2, e1 == any(1), any(nan) == any(nan), any(zero) == any(negzero))
	try := func(f func()) {
		defer func() { fmt.Println("recovered:", recover()) }()
		f()
	}
	try(func() { mi[[]int{1}] = 1 })
	try(func() { var a, b any = []int{1}, []int{1}; fmt.Println(a == b) })
	try(func() { _ = mi[map[int]int{}] })
	try(func() { var a, b any = struct{ s []int }{}, struct{ s []int }{}; fmt.Println(a == b) })
	try(func() { var a, b any = [1]func(){}, [1]func(){}; fmt.Println(a == b) })
	try(func() { var a any = 1; var b any = []int{}; fmt.Println(a == b) })
	fmt.Println(min(nan, 1), max(nan, 1.0), min(negzero, zero), max(negzero, zero), 1/min(negzero, zero), 1/max(negzero, zero), min(1, 2.5, -1), max("a", "b", ""))
	fmt.Println(nan == nan, nan < 1, nan > 1, nan != nan, zero == negzero, -zero < zero, nan >= nan)
	s := []int{1, 2, 3}
	clear(s)
	fmt.Println(s)
}`)
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

// floatFor returns a float64 constant whose truncation fits the integer type t.
func (g *Gen) floatFor(t *Type) string {
	if t.Bits == 64 {
		switch g.rn(5) {
		case 0:
			if t.Signed {
				return pickS(g, "9223372036854774784", "-9223372036854775808", "-9223372036854774784")
			}
			return pickS(g, "18446744073709549568", "9223372036854775808", "9223372036854774784")
		}
		n := int64(g.rn(1<<30))<<22 + int64(g.rn(1<<22))
		if t.Signed && g.chance(50) {
			n = -n
		}
		return g.withFrac(n)
	}
	raw := g.intRaw(t)
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		u, _ := strconv.ParseUint(raw, 10, 64)
		n = int64(u)
	}
	return g.withFrac(n)
}

func pickS(g *Gen, list ...string) string { return list[g.rn(len(list))] }

// withFrac adds a fraction below 1 to n, away from zero, so that the value truncates to n.
func (g *Gen) withFrac(n int64) string {
	fr := []float64{0, 0.25, 0.5, 0.75, 0.9375, 0.0625}[g.rn(6)]
	v := float64(n)
	if n < 0 || (n == 0 && g.chance(50)) {
		v -= fr
	} else {
		v += fr
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func scenFloatConv(g *Gen) {
	var body strings.Builder
	for _, tn := range []string{"int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "int", "uint"} {
		t := intTs[tn]
		for i := g.between(1, 3); i > 0; i-- {
			fmt.Fprintf(&body, "\tf = %s\n\tfmt.Println(f, %s(f), %s(float32(f)), -f, %s(-f))\n", g.floatFor(t), tn, tn, tn)
		}
	}
	g.tpl(`{
	var f float64
@BODY@
	for _, v := range []int64{@I1@, @I2@, 1<<53 + 1, 1<<62 + 1<<9 + 1, -1<<63, 1<<63 - 1} {
		fmt.Println(float64(v), float32(v), float64(uint64(v)), float32(uint64(v)), float64(int32(v)), float32(uint32(v)))
	}
	var u8 uint8 = @U8@
	var i16 int16 = @I16@
	fmt.Println(float64(u8)/3, float32(i16)/7, float32(float64(u8))*0.1, float64(float32(0.1)))
}`, "BODY", strings.TrimRight(body.String(), "\n"), "I1", g.intRaw(intTs["int64"]), "I2", g.intRaw(intTs["int64"]),
		"U8", g.intRaw(intTs["uint8"]), "I16", g.intRaw(intTs["int16"]))
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenFloatOOR(g *Gen) {
	vals := []string{"1e10", "-1e10", "3e9", "4294967296", "4294967295.5", "1e19", "-1e19", "1.8446744073709552e19", "9223372036854775808",
		"-9223372036854775809", "256.5", "-1.5", "-0.5", "65536", "-129", "128", "32768", "255.5", "2147483648", "-2147483649", "1e300", "-1e300"}
	var picked []string
	for i := g.between(6, 10); i > 0; i-- {
		picked = append(picked, pickS(g, vals...))
	}
	g.tpl(`{
	var zero float64
	for _, f := range []float64{@V@, 1/zero, -1/zero, zero/zero} {
		fmt.Println(f, int8(f), uint8(f), int16(f), uint16(f), int32(f), uint32(f), int64(f), uint64(f), int(f), uint(f))
		g := float32(f)
		fmt.Println(g, int8(g), uint8(g), int16(g), uint16(g), int32(g), uint32(g), int64(g), uint64(g), int(g), uint(g))
	}
}`, "V", strings.Join(picked, ", "))
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenUTF8Bad(g *Gen) {
	g.importPkg("unicode/utf8")
	list := []string{`"\xe0\x80\x80"`, `"\xe0\x9f\xbf"`, `"\xe0\xa0\x80"`, `"\xed\x9f\xbf"`, `"\xed\xa0\x80"`, `"\xed\xbf\xbf"`, `"\xef\xbf\xbe"`,
		`"\xf0\x80\x80\x80"`, `"\xf0\x8f\xbf\xbf"`, `"\xf0\x90\x80\x80"`, `"\xf4\x8f\xbf\xbf"`, `"\xf4\x90\x80\x80"`, `"\xf5\x80\x80\x80"`,
		`"\xc0\xaf"`, `"\xc1\xbf"`, `"\xc2\x80"`, `"\xf8\x88\x80\x80\x80"`, `"\xfc\x84\x80\x80\x80\x80"`, `"\xe2\x82"`, `"\xf0\x9f\x98"`,
		`"\x80"`, `"\xbf"`, `"\xff"`, `"a\xe0\x80b"`, `"\xe0\x80\x80\xe0\x80\x80"`, `"é\xe0\x9f\xbfx"`, `"\xed\xa0\x80\xed\xb0\x80"`, `"\xf4\x90"`, `"\xe0"`, `"\xf0"`, `"\xf4"`}
	var picked []string
	for i := g.between(8, 16); i > 0; i-- {
		picked = append(picked, pickS(g, list...))
	}
	g.tpl(`{
	for _, s := range []string{@LIST@} {
		for i, r := range s {
			fmt.Print(i, ":", r, " ")
		}
		fmt.Println(len(s))
		rs := []rune(s)
		r1, sz1 := utf8.DecodeRuneInString(s)
		r2, sz2 := utf8.DecodeLastRuneInString(s)
		fmt.Println(rs, len(rs), string(rs) == s, []byte(string(rs)), utf8.ValidString(s), utf8.RuneCountInString(s), r1, sz1, r2, sz2, utf8.FullRuneInString(s))
		fmt.Printf("%q %+q %x % x %U\n", s, s, s, s, rs)
		b := []byte(s)
		fmt.Println(utf8.Valid(b), utf8.RuneCount(b), utf8.FullRune(b), utf8.AppendRune(nil, r1))
		for len(b) > 0 {
			r, n := utf8.DecodeRune(b)
			fmt.Print(r, "/", n, " ")
			b = b[n:]
		}
		fmt.Println()
	}
	for _, r := range []rune{0xD7FF, 0xD800, 0xDFFF, 0xE000, 0xFFFD, 0x10FFFF, 0x110000, -1, 0x7FFFFFFF, 0x7F, 0x80, 0x7FF, 0x800, 0xFFFF, 0x10000} {
		buf := utf8.AppendRune(nil, r)
		fmt.Println(r, buf, string(r) == string(rune(0xFFFD)), utf8.RuneLen(r), utf8.ValidRune(r), []byte(string(r)))
	}
}`, "LIST", strings.Join(picked, ", "))
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenPrintln(g *Gen) {
	f1, f2 := g.floatRaw(), g.floatRaw()
	g.tpl(`{
	println("p", 1, true, -5, uint8(200), int64(-1<<63), "é\xff", @F1@, float32(@F2@), 'x', int8(-3), uint64(1<<63), uint16(65535))
	println()
	print("x", 7, true, 2.5, "\n")
	print(@F1@, "\n")
	println(complex(@F1@, @F2@))
	println(@F1@ + @F2@)
	var z float64
	println(1/z, -1/z, z/z, -z)
	println(@S@)
	var np *int
	println(np == nil)
}`, "F1", f1, "F2", f2, "S", g.strRaw())
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenCapGrowth(g *Gen) {
	caps, capn := g.topName("caps"), g.topName("capn")
	g.topf("func %s[T any](n int) []int {\n\tvar s []T\n\tvar out []int\n\tlast := -1\n\tfor i := 0; i < n; i++ {\n\t\tvar z T\n\t\ts = append(s, z)\n\t\tif cap(s) != last {\n\t\t\tlast = cap(s)\n\t\t\tout = append(out, last)\n\t\t}\n\t}\n\treturn out\n}\n", caps)
	g.topf("func %s[T any](n int) int { return cap(append([]T(nil), make([]T, n)...)) }\n", capn)
	for _, ty := range []string{"int", "byte", "int32", "string", "any", "[]int", "struct{}", "*int", "[3]byte", "float64", "struct{ a, b int }", "[100]int", "struct {\n\t\tp *int\n\t\tn int\n\t}", "struct {\n\t\ta string\n\t\tb int8\n\t}"} {
		g.ln("fmt.Println(%q, %s[%s](%d), %s[%s](1), %s[%s](5), %s[%s](33), %s[%s](100))", strings.Join(strings.Fields(ty), " "), caps, ty, g.between(300, 900), capn, ty, capn, ty, capn, ty, capn, ty)
	}
	g.showAll()
}
