package gen

import (
	"fmt"
	"strconv"
	"strings"
)

func init() {
	register("ints", 14, []string{"ints"}, scenInts)
	register("floats", 5, []string{"floats"}, scenFloats)
	register("strings", 9, []string{"strings"}, scenStrings)
	register("slices", 9, []string{"slices"}, scenSlices)
	register("slicemodel", 9, []string{"slices"}, scenSliceModel)
	register("arrays", 8, []string{"arrays"}, scenArrays)
	register("structs", 8, []string{"structs"}, scenStructs)
	register("maps", 9, []string{"maps"}, scenMaps)
	register("pointers", 8, []string{"pointers", "structs"}, scenPointers)
	register("consts", 6, []string{"consts"}, scenConsts)
	register("labels", 7, []string{"labels"}, scenLabels)
	register("multiret", 6, []string{"multiret"}, scenMulti)
	register("fmtdeep", 8, []string{"fmtverbs"}, scenFmtDeep)
	register("soup", 12, nil, scenSoup)
}

func scenSoup(g *Gen) {
	g.seedVars(g.between(3, 7))
	g.stmts(g.between(10, 24), 3)
	g.showAll()
}

func scenInts(g *Gen) {
	g.seedBasic(g.between(3, 7))
	g.stmts(g.between(6, 16), 2)
	for i := g.between(2, 5); i > 0; i-- {
		t := g.randIntType()
		g.show(g.expr(t, g.between(2, 4)), t)
	}
	// the corners: MinInt / -1, shifts by the width or more, conversions that wrap
	t := g.randIntType()
	lo, hi := intRange(t)
	v := g.declare("m", t, fmt.Sprintf("%s(%s)", t.Name, paren(lo)))
	h := g.declare("h", t, fmt.Sprintf("%s(%s)", t.Name, hi))
	m1 := g.declare("n", t, fmt.Sprintf("^%s(0)", t.Name))
	g.ln("fmt.Println(%s/%s, %s%%%s, -%s, %s*%s, %s+1, %s-1)", v.Name, m1.Name, v.Name, m1.Name, v.Name, h.Name, h.Name, h.Name, v.Name)
	for _, c := range []string{"0", "1", fmt.Sprint(t.Bits - 1), fmt.Sprint(t.Bits), fmt.Sprint(t.Bits + 1), "200", "1000"} {
		cnt := g.declare("c", intTs["uint"], "uint("+c+")")
		g.ln("fmt.Println(%s<<%s, %s>>%s, %s>>%s)", h.Name, cnt.Name, h.Name, cnt.Name, v.Name, cnt.Name)
	}
	for _, o := range []string{"int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "int", "uint"} {
		g.ln("fmt.Println(%s(%s), %s(%s))", o, h.Name, o, v.Name)
	}
	g.showAll()
}

func scenFloats(g *Gen) {
	n := g.between(2, 5)
	for i := 0; i < n; i++ {
		t := tF64
		if g.chance(35) {
			t = tF32
		}
		g.declareVal("f", t)
	}
	g.declareVal("i", intTs[pick(g, []string{"int8", "int16", "int32", "uint8", "uint16"})])
	g.stmts(g.between(4, 10), 2)
	for i := g.between(2, 4); i > 0; i-- {
		t := tF64
		if g.chance(35) {
			t = tF32
		}
		g.show(g.expr(t, 3), t)
	}
	// special values from arithmetic: infinities, NaN, negative zero
	z := g.declare("z", tF64, "float64(0)")
	one := g.declare("o", tF64, "float64(1)")
	g.ln("fmt.Println(%s/%s, -%s/%s, %s/%s == %s/%s, -%s)", one.Name, z.Name, one.Name, z.Name, z.Name, z.Name, z.Name, z.Name, z.Name)
	g.ln("fmt.Println(%s/%s < %s, %s/%s > %s)", z.Name, z.Name, one.Name, z.Name, z.Name, one.Name)
	g.showAll()
}

// ---- strings ----

func scenStrings(g *Gen) {
	lit := g.strNonEmpty()
	s := g.declare("s", tString, lit)
	if u, err := strconv.Unquote(lit); err == nil {
		s.MinLen = len(u)
	}
	t := g.declare("t", tString, g.strRaw())
	g.ln("fmt.Println(len(%s), len(%s), %s == %s, %s < %s, %s+%s)", s.Name, t.Name, s.Name, t.Name, s.Name, t.Name, s.Name, t.Name)
	g.enter(fmt.Sprintf("for i, r := range %s", s.Name))
	g.ln("fmt.Println(i, r, string(r), r == 0xFFFD)")
	g.leave()
	g.enter(fmt.Sprintf("for i := 0; i < len(%s); i++", s.Name))
	g.ln("fmt.Print(%s[i], \" \")", s.Name)
	g.leave()
	g.ln("fmt.Println()")
	g.ln("fmt.Println([]byte(%s), []rune(%s))", s.Name, s.Name)
	g.ln("fmt.Printf(\"%%q %%x %%X %% x %%+q %%#q\\n\", %s, %s, %s, %s, %s, %s)", s.Name, s.Name, s.Name, s.Name, s.Name, s.Name)
	rs := g.declare("rs", sliceOf(intTs["int32"]), "[]rune("+s.Name+")")
	g.ln("fmt.Println(len(%s), string(%s), string(%s[len(%s)/2:]))", rs.Name, rs.Name, rs.Name, rs.Name)
	bs := g.declare("bs", sliceOf(intTs["uint8"]), "[]byte("+s.Name+")")
	g.enter(fmt.Sprintf("for i := range %s", bs.Name))
	g.ln("%s[i] ^= byte(i)", bs.Name)
	g.leave()
	g.ln("fmt.Println(string(%s), %s)", bs.Name, s.Name)
	// runes
	for _, r := range []string{"0", "65", "0xe9", "0x65e5", "0x1F600", "0xD800", "0x10FFFF", "0x110000", "-1"} {
		rv := g.declare("r", intTs["int32"], "int32("+r+")")
		g.ln("fmt.Printf(\"%%q %%U %%d %%x %%v|\", string(rune(%s)), %s, %s, string(rune(%s)), string(rune(%s)))", rv.Name, rv.Name, rv.Name, rv.Name, rv.Name)
	}
	g.ln("fmt.Println()")
	// building and switching
	acc := g.declare("acc", tString, `""`)
	g.enter(fmt.Sprintf("for _, r := range %s + %s", s.Name, t.Name))
	g.enter("switch")
	g.ln("case r < 128:")
	g.ln("\t%s += \"a\"", acc.Name)
	g.ln("case r < 0x800:")
	g.ln("\t%s += \"b\"", acc.Name)
	g.ln("default:")
	g.ln("\t%s += string(r)", acc.Name)
	g.leave()
	g.leave()
	g.ln("fmt.Println(%s)", acc.Name)
	// byte-wise and rune-wise counting into a map
	cnt := g.declare("cnt", mapOf(intTs["int32"], intTs["int"]), "map[int32]int{}")
	g.enter(fmt.Sprintf("for _, r := range %s", s.Name))
	g.ln("%s[r]++", cnt.Name)
	g.leave()
	g.ln("fmt.Println(%s)", cnt.Name)
	if g.chance(60) {
		n := g.between(0, 3)
		g.ln("fmt.Println(%s[:%d], %s[%d:], %s[%d:%d])", "\"héllo, 日本\"", n, "\"héllo, 日本\"", n+1, "\"héllo, 日本\"", n, n+2)
	}
	g.stmts(g.between(0, 6), 1)
	g.showAll()
}

// ---- slices ----

type sl struct {
	name     string
	len, cap int // cap < 0: unknown (the slice owns a grown array nobody else shares)
	shared   bool
}

func scenSliceModel(g *Gen) {
	et := g.randType(1)
	if et.K == KMap {
		et = g.randBasic()
	}
	T := sliceOf(et)
	n := g.between(2, 6)
	c := n + g.between(0, 4)
	base := &sl{name: g.fresh("b"), len: n, cap: c}
	g.ln("%s := make(%s, %d, %d)", base.name, T.Name, n, c)
	g.enter(fmt.Sprintf("for i := range %s", base.name))
	if et.isBasic() && et.K == KInt {
		g.ln("%s[i] = %s(i*%d + %s)", base.name, et.Name, g.between(1, 9), g.intRaw(intTs["int8"]))
	} else {
		g.ln("%s[i] = %s", base.name, g.val(et, 1))
	}
	g.leave()
	all := []*sl{base}
	g.bindSl(base, T)
	show := func(s *sl) {
		if s.cap >= 0 {
			g.ln("fmt.Println(%q, len(%s), cap(%s), %s)", s.name, s.name, s.name, s.name)
		} else {
			g.ln("fmt.Println(%q, len(%s), %s)", s.name, s.name, s.name)
		}
	}
	show(base)
	pickSl := func(known bool) *sl {
		var cs []*sl
		for _, s := range all {
			if !known || s.cap >= 0 {
				cs = append(cs, s)
			}
		}
		if len(cs) == 0 {
			return nil
		}
		return pick(g, cs)
	}
	for i := g.between(4, 12); i > 0; i-- {
		switch g.roll(20, 18, 14, 10, 10, 10, 8, 10) {
		case 0: // sub-slice, aliasing
			s := pickSl(true)
			if s == nil {
				break
			}
			lo := g.rn(s.len + 1)
			hi := lo + g.rn(s.len-lo+1)
			ns := &sl{name: g.fresh("w"), len: hi - lo, cap: s.cap - lo, shared: true}
			if g.chance(40) {
				m := hi + g.rn(s.cap-hi+1)
				g.ln("%s := %s[%d:%d:%d]", ns.name, s.name, lo, hi, m)
				ns.cap = m - lo
			} else {
				g.ln("%s := %s[%d:%d]", ns.name, s.name, lo, hi)
			}
			g.bindSl(ns, T)
			all = append(all, ns)
			show(ns)
		case 1: // append one element: within capacity it writes the shared array
			s := pickSl(false)
			g.ln("%s = append(%s, %s)", s.name, s.name, g.val(et, 1))
			s.len++
			if s.cap >= 0 && s.len > s.cap {
				s.cap = -1 // grew into a fresh array of unspecified capacity
			}
			show(s)
		case 2: // set elements through a slice, observe through the others
			s := pickSl(false)
			if s.len > 0 {
				k := g.rn(s.len)
				g.ln("%s[%d] = %s", s.name, k, g.val(et, 1))
				for _, o := range all {
					if o.cap >= 0 || o == s {
						g.ln("fmt.Println(%q, %s)", o.name, o.name)
					}
				}
			}
		case 3: // copy with overlap
			s := pickSl(false)
			t := pickSl(false)
			if g.chance(50) && s.len > 1 {
				// the two windows of one array overlap, the destination after the source or
				// before it: the result is as if the source were read first
				k := 1 + g.rn(s.len-1)
				if g.chance(50) {
					g.ln("fmt.Println(copy(%s[%d:], %s), %s)", s.name, k, s.name, s.name)
				} else {
					g.ln("fmt.Println(copy(%s, %s[%d:]), %s)", s.name, s.name, k, s.name)
				}
				break
			}
			g.ln("fmt.Println(copy(%s, %s), %s, %s)", s.name, t.name, s.name, t.name)
		case 4: // append a slice to a fresh nil slice
			if g.chance(50) {
				s := pickSl(false)
				nn := &sl{name: g.fresh("a"), len: s.len, cap: -1}
				g.ln("var %s %s", nn.name, T.Name)
				g.ln("%s = append(%s, %s...)", nn.name, nn.name, s.name)
				g.bindSl(nn, T)
				all = append(all, nn)
				g.ln("fmt.Println(%q, len(%s), %s == nil)", nn.name, nn.name, nn.name)
			}
		case 5: // delete an element in place
			s := pickSl(false)
			if s.len > 0 && s.cap >= 0 {
				k := g.rn(s.len)
				g.ln("%s = append(%s[:%d], %s[%d:]...)", s.name, s.name, k, s.name, k+1)
				s.len--
				show(s)
				g.ln("fmt.Println(%s)", base.name)
			}
		case 6: // reverse in place
			s := pickSl(false)
			g.enter(fmt.Sprintf("for i, j := 0, len(%s)-1; i < j; i, j = i+1, j-1", s.name))
			g.ln("%s[i], %s[j] = %s[j], %s[i]", s.name, s.name, s.name, s.name)
			g.leave()
			show(s)
		case 7: // append with a three-index slice forces a copy
			s := pickSl(true)
			if s != nil && s.len > 0 {
				ns := &sl{name: g.fresh("x"), len: s.len + 1, cap: -1}
				g.ln("%s := append(%s[:%d:%d], %s)", ns.name, s.name, s.len, s.len, g.val(et, 1))
				g.bindSl(ns, T)
				all = append(all, ns)
				g.ln("fmt.Println(%q, len(%s), %s, %s)", ns.name, ns.name, ns.name, s.name)
			}
		}
	}
	for _, s := range all {
		g.ln("fmt.Println(%q, %s)", s.name, s.name)
	}
}

func (g *Gen) bindSl(s *sl, t *Type) {
	v := g.bind(s.name, t)
	v.MinLen = s.len
	v.NoRead = true // the model owns it: expressions must not use it
	v.Used = true
}

func scenSlices(g *Gen) {
	g.seedVars(g.between(2, 4))
	for i := g.between(1, 3); i > 0; i-- {
		g.declareVal("sl", sliceOf(g.randType(1)))
	}
	g.stmts(g.between(8, 18), 3)
	g.showAll()
}

// ---- arrays ----

func scenArrays(g *Gen) {
	et := g.randType(1)
	if et.K == KMap || et.K == KSlice {
		et = g.randBasic()
	}
	n := g.between(2, 5)
	A := arrayOf(et, n)
	a := g.declareVal("a", A)
	b := g.declare("b", A, a.Name)
	g.ln("%s[%d] = %s", b.Name, g.rn(n), g.val(et, 1))
	g.ln("fmt.Println(%s, %s)", a.Name, b.Name)
	if A.comparable() {
		g.ln("fmt.Println(%s == %s, %s != %s)", a.Name, b.Name, a.Name, b.Name)
	}
	// pass by value, return by value
	fn := g.topName("arr")
	g.topFunc(fmt.Sprintf("%s(x %s) %s", fn, A.Name, A.Name), []*Var{{Name: "x", T: A}}, func() {
		g.ln("x[%d] = %s", g.rn(n), g.val(et, 1))
		g.sweep()
		g.ln("return x")
	})
	r := g.declare("r", A, fmt.Sprintf("%s(%s)", fn, a.Name))
	g.ln("fmt.Println(%s, %s)", a.Name, r.Name)
	// range copies the array
	g.enter(fmt.Sprintf("for i, v := range %s", a.Name))
	g.ln("%s[(i+1)%%%d] = %s", a.Name, n, g.val(et, 1))
	g.ln("fmt.Println(i, v, %s)", a.Name)
	g.leave()
	// slicing an array aliases it
	if g.has("slices") {
		s := g.declare("s", sliceOf(et), a.Name+"[:]")
		g.ln("%s[0] = %s", s.Name, g.val(et, 1))
		lo := g.rn(n)
		g.ln("fmt.Println(%s, len(%s), cap(%s), len(%s[%d:]), cap(%s[%d:]))", a.Name, s.Name, s.Name, s.Name, lo, s.Name, lo)
		g.ln("%s = append(%s, %s)", s.Name, s.Name, g.val(et, 1))
		g.ln("%s[1] = %s", s.Name, g.val(et, 1))
		g.ln("fmt.Println(%s, %s)", a.Name, s.Name)
	}
	// pointer to array
	if g.has("pointers") {
		p := g.declare("p", ptrTo(A), "&"+a.Name)
		g.ln("%s[%d] = %s", p.Name, g.rn(n), g.val(et, 1))
		g.ln("for i := range %s { %s[i] = %s[(i+1)%%%d] }", p.Name, b.Name, p.Name, n)
		g.ln("fmt.Println(len(%s), %s, %s)", p.Name, a.Name, b.Name)
		if A.comparable() {
			g.ln("fmt.Println(*%s == %s)", p.Name, a.Name)
		}
	}
	// arrays of arrays, and in structs
	if g.has("structs") {
		st := g.newStructWith([]Field{{Name: "Arr", T: A}, {Name: "N", T: intTs["int8"]}})
		s1 := g.declare("st", st, fmt.Sprintf("%s{Arr: %s, N: 7}", st.Name, a.Name))
		s2 := g.declare("st", st, s1.Name)
		g.ln("%s.Arr[0] = %s", s2.Name, g.val(et, 1))
		g.ln("fmt.Printf(\"%%v %%+v\\n\", %s, %s)", s1.Name, s2.Name)
		if st.comparable() {
			g.ln("fmt.Println(%s == %s)", s1.Name, s2.Name)
		}
	}
	m := g.declare("mm", arrayOf(arrayOf(intTs["int16"], 2), 3), "[3][2]int16{}")
	g.ln("%s[1][1] = %s", m.Name, g.lit(intTs["int16"]))
	row := g.declare("row", arrayOf(intTs["int16"], 2), m.Name+"[1]")
	g.ln("%s[0]++", row.Name)
	g.ln("fmt.Println(%s, %s)", m.Name, row.Name)
	g.ln("fmt.Println(len(%s), len(%s[0]))", m.Name, m.Name)
	g.stmts(g.between(0, 6), 1)
	g.showAll()
}

// newStructWith declares a struct type with the given fields.
func (g *Gen) newStructWith(fs []Field) *Type {
	name := g.topName("T")
	t := &Type{K: KStruct, Name: name, Named: true, NoPtr: true}
	var b strings.Builder
	fmt.Fprintf(&b, "type %s struct {\n", name)
	for _, f := range fs {
		t.Fields = append(t.Fields, f)
		t.NoPtr = t.NoPtr && f.T.NoPtr
		if f.Embedded {
			fmt.Fprintf(&b, "\t%s\n", f.T.Name)
		} else {
			fmt.Fprintf(&b, "\t%s %s\n", f.Name, f.T.Name)
		}
	}
	b.WriteString("}\n")
	g.top.WriteString(b.String())
	g.types = append(g.types, t)
	return t
}

// ---- structs ----

func scenStructs(g *Gen) {
	t1 := g.newStruct(2)
	t2 := g.newStruct(2)
	// a struct that holds another
	outer := g.newStructWith([]Field{{Name: "In", T: t1}, {Name: "Z", T: t2}, {Name: "K", T: intTs["uint16"]}})
	v1 := g.declareVal("v", t1)
	v2 := g.declareVal("v", outer)
	v3 := g.declare("v", outer, v2.Name)
	g.ln("%s.K += 5", v3.Name)
	g.ln("%s.In = %s", v3.Name, v1.Name)
	g.show(v2.Name, outer)
	g.show(v3.Name, outer)
	if outer.comparable() {
		g.ln("fmt.Println(%s == %s, %s.In == %s)", v2.Name, v3.Name, v3.Name, v1.Name)
	}
	// anonymous struct values
	g.ln("anon := struct{ A int8; B string }{%s, %s}", g.lit(intTs["int8"]), g.strRaw())
	g.ln("anon2 := anon")
	g.ln("anon2.A--")
	g.ln("fmt.Printf(\"%%v %%+v %%#v %%T %%v\\n\", anon, anon2, anon, anon, anon == anon2)")
	// map of structs: read-modify-write
	if g.has("maps") {
		m := g.declare("m", mapOf(tString, t1), "map[string]"+t1.Name+"{}")
		g.ln("%s[\"a\"] = %s", m.Name, v1.Name)
		g.ln("%s[\"b\"] = %s", m.Name, v1.Name)
		if t1.comparable() {
			g.ln("fmt.Println(len(%s), %s[\"a\"] == %s[\"b\"])", m.Name, m.Name, m.Name)
		}
	}
	// struct copy in function call and return
	fn := g.topName("st")
	g.topFunc(fmt.Sprintf("%s(x %s) (y %s, n int)", fn, outer.Name, outer.Name), []*Var{{Name: "x", T: outer}}, func() {
		g.ln("x.K = x.K*3 + 1")
		g.ln("y = x")
		g.ln("y.K++")
		g.ln("n = int(x.K)")
		g.ln("return")
	})
	g.ln("y1, n1 := %s(%s)", fn, v2.Name)
	g.ln("fmt.Println(y1.K, n1, %s.K)", v2.Name)
	g.stmts(g.between(2, 8), 2)
	g.showAll()
}

// ---- maps ----

func scenMaps(g *Gen) {
	kt := g.randKey()
	vt := g.randType(1)
	M := mapOf(kt, vt)
	nk := distinctKeyCount(kt)
	m := g.declareVal("m", M)
	g.ln("fmt.Println(len(%s), %s)", m.Name, m.Name)
	for i := g.between(3, 8); i > 0; i-- {
		k := g.keyLit(kt, g.rn(nk))
		switch g.roll(40, 20, 25, 15) {
		case 0:
			g.ln("%s[%s] = %s", m.Name, k, g.val(vt, 1))
		case 1:
			g.ln("delete(%s, %s)", m.Name, k)
		case 2:
			e, ok := g.fresh("e"), g.fresh("ok")
			g.ln("%s, %s := %s[%s]", e, ok, m.Name, k)
			g.show(e, vt)
			g.ln("fmt.Println(%s)", ok)
			g.bind(e, vt).Used = true
			g.bind(ok, tBool).Used = true
		default:
			g.ln("fmt.Println(len(%s), %s)", m.Name, m.Name)
		}
	}
	// counting, and read-modify-write of elements
	if vt.K == KInt || vt.K == KString || vt.K == KFloat {
		for i := g.between(1, 4); i > 0; i-- {
			k := g.keyLit(kt, g.rn(nk))
			switch vt.K {
			case KInt:
				g.ln("%s[%s]++", m.Name, k)
				g.ln("%s[%s] += %s", m.Name, k, g.lit(vt))
			case KString:
				g.ln("%s[%s] += %s", m.Name, k, g.strRaw())
			case KFloat:
				g.ln("%s[%s] += %s", m.Name, k, g.lit(vt))
			}
		}
	}
	if vt.K == KSlice && vt.Elem.K == KInt {
		k := g.keyLit(kt, g.rn(nk))
		g.ln("%s[%s] = append(%s[%s], %s)", m.Name, k, m.Name, k, g.lit(vt.Elem))
	}
	if vt.K == KStruct || vt.K == KArray {
		k := g.keyLit(kt, g.rn(nk))
		g.ln("if v, ok := %s[%s]; ok {", m.Name, k)
		g.ln("\t%s[%s] = v", m.Name, k)
		g.ln("}")
	}
	// aggregation over a range loop is order independent
	g.ln("agg := 0")
	g.enterRaw(fmt.Sprintf("for k, v := range %s { %s", m.Name, orderIndependent))
	g.ln("agg += len(fmt.Sprint(k)) * 3 + len(fmt.Sprint(v))")
	g.leave()
	g.ln("fmt.Println(agg, len(%s))", m.Name)
	// delete the current key while ranging
	g.ln("seen := 0")
	g.enterRaw(fmt.Sprintf("for k := range %s { %s", m.Name, orderIndependent))
	g.ln("delete(%s, k)", m.Name)
	g.ln("seen++")
	g.leave()
	g.ln("fmt.Println(seen, len(%s), %s)", m.Name, m.Name)
	// a nil map: reads, len, delete and range are fine
	g.ln("var nm %s", M.Name)
	g.ln("_, nok := nm[%s]", g.keyLit(kt, 0))
	g.ln("delete(nm, %s)", g.keyLit(kt, 1))
	g.ln("fmt.Println(len(nm), nok, nm == nil, nm)")
	g.enterRaw("for range nm { " + orderIndependent)
	g.ln("fmt.Println(\"never\")")
	g.leave()
	// nested maps
	g.ln("nested := map[string]map[int8]int{}")
	g.ln("for i := 0; i < 6; i++ {")
	g.ln("\tkey := string(rune('a' + i%%3))")
	g.ln("\tif nested[key] == nil { nested[key] = map[int8]int{} }")
	g.ln("\tnested[key][int8(i)] += i * 2")
	g.ln("}")
	g.ln("fmt.Println(nested, len(nested))")
	g.stmts(g.between(0, 6), 1)
	g.showAll()
}

// ---- pointers ----

func scenPointers(g *Gen) {
	st := g.newStruct(1)
	x := g.declareVal("x", intTs[pick(g, []string{"int", "int8", "uint16", "int64"})])
	px := g.declare("p", ptrTo(x.T), "&"+x.Name)
	py := g.declare("q", ptrTo(x.T), "&"+x.Name)
	g.ln("*%s += 3", px.Name)
	g.ln("*%s *= 2", py.Name)
	g.ln("fmt.Println(%s, *%s, *%s, %s == %s)", x.Name, px.Name, py.Name, px.Name, py.Name)
	y := g.declareVal("y", x.T)
	g.ln("fmt.Println(&%s == &%s, &%s == %s)", x.Name, y.Name, x.Name, px.Name)
	// pointer to a pointer
	g.ln("pp := &%s", px.Name)
	g.ln("**pp = %s(7)", x.T.Name)
	g.ln("fmt.Println(%s)", x.Name)
	// swap through pointers
	sw := g.topName("swap")
	g.topFunc(fmt.Sprintf("%s(a, b *%s)", sw, x.T.Name), nil, func() {
		g.ln("*a, *b = *b, *a")
	})
	g.ln("%s(&%s, &%s)", sw, x.Name, y.Name)
	g.ln("fmt.Println(%s, %s)", x.Name, y.Name)
	// pointers to struct fields, and new
	s := g.declareVal("s", st)
	ps := g.declare("ps", ptrTo(st), "&"+s.Name)
	var ps2 []Path
	g.walk(&Var{Name: ps.Name, T: ps.T, NonNil: true}, ps.Name, ps.T, 0, &ps2)
	for _, p := range ps2 {
		if p.T.K == KInt && p.E != ps.Name {
			f := g.declare("fp", ptrTo(p.T), "&"+p.E)
			g.ln("*%s += 1", f.Name)
			break
		}
	}
	g.ln("fmt.Println(%s, *%s)", s.Name, ps.Name)
	np := g.declare("np", ptrTo(st), "new("+st.Name+")")
	g.ln("*%s = %s", np.Name, s.Name)
	g.ln("fmt.Println(*%s, %s == %s, %s == nil)", np.Name, np.Name, ps.Name, np.Name)
	if st.comparable() {
		g.ln("fmt.Println(*%s == %s)", np.Name, s.Name)
	}
	// elements of slices and arrays
	if g.has("slices") {
		sl := g.declare("sl", sliceOf(intTs["int16"]), "[]int16{1, 2, 3, 4}")
		pe := g.declare("pe", ptrTo(intTs["int16"]), "&"+sl.Name+"[2]")
		g.ln("*%s = 99", pe.Name)
		g.ln("%s = append(%s[:1], %s[2:]...)", sl.Name, sl.Name, sl.Name)
		g.ln("fmt.Println(%s, *%s)", sl.Name, pe.Name)
	}
	// a list and a tree, no cycles
	ln := g.topName("Node")
	g.topf("type %s struct {\n\tV int8\n\tNext *%s\n}", ln, ln)
	g.ln("var head *%s", ln)
	g.enter(fmt.Sprintf("for i := %d; i > 0; i--", g.between(2, 6)))
	g.ln("head = &%s{V: int8(i * %d), Next: head}", ln, g.between(1, 40))
	g.leave()
	g.ln("sum := int8(0)")
	g.enter("for n := head; n != nil; n = n.Next")
	g.ln("sum += n.V")
	g.ln("fmt.Print(n.V, \",\")")
	g.leave()
	g.ln("fmt.Println(\"sum\", sum)")
	tn := g.topName("Tree")
	g.topf("type %s struct {\n\tL, R *%s\n\tK int16\n}", tn, tn)
	ins := g.topName("insert")
	g.topf("func %s(t *%s, k int16) *%s {\n\tif t == nil {\n\t\treturn &%s{K: k}\n\t}\n\tif k < t.K {\n\t\tt.L = %s(t.L, k)\n\t} else {\n\t\tt.R = %s(t.R, k)\n\t}\n\treturn t\n}\n", ins, tn, tn, tn, ins, ins)
	walkf := g.topName("walk")
	g.topf("func %s(t *%s, depth int) {\n\tif t == nil {\n\t\treturn\n\t}\n\t%s(t.L, depth+1)\n\tfmt.Print(t.K, \"@\", depth, \" \")\n\t%s(t.R, depth+1)\n}\n", walkf, tn, walkf, walkf)
	g.ln("var root *%s", tn)
	g.ln("for _, k := range []int16{%s} {", strings.Join(g.randInts("int16", g.between(3, 8)), ", "))
	g.ln("\troot = %s(root, k)", ins)
	g.ln("}")
	g.ln("%s(root, 0)", walkf)
	g.ln("fmt.Println()")
	g.stmts(g.between(0, 5), 1)
	g.showAll()
}

func (g *Gen) randInts(tn string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = paren(g.intRaw(intTs[tn]))
	}
	return out
}

// ---- constants ----

func scenConsts(g *Gen) {
	// an iota block with implicit repetition
	nm := g.between(3, 7)
	base := g.topName("K")
	exprs := []string{"iota", "iota + 1", "1 << iota", "iota * 10", "iota * iota", "1 << (iota * 2)", "-iota", "iota%2", "(iota + 1) * 3 - 1"}
	var typ string
	if g.chance(60) {
		typ = " " + pick(g, []string{"int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "int", "uint", "float64"})
		if strings.Contains(typ, "uint") {
			exprs = []string{"iota", "iota + 1", "1 << iota", "iota * 10", "iota * iota", "iota%2"}
		}
	}
	if strings.Contains(typ, "8") {
		var ex []string
		for _, e := range exprs {
			if e != "1 << (iota * 2)" {
				ex = append(ex, e)
			}
		}
		exprs = ex
	}
	var b strings.Builder
	b.WriteString("const (\n")
	first := pick(g, exprs)
	var names []int
	for i := 0; i < nm; i++ {
		name := fmt.Sprintf("%s_%d", base, i)
		switch {
		case i == 0:
			fmt.Fprintf(&b, "\t%s%s = %s\n", name, typ, first)
			names = append(names, i)
		case g.chance(15):
			fmt.Fprintf(&b, "\t_\n")
		default:
			fmt.Fprintf(&b, "\t%s\n", name)
			names = append(names, i)
		}
	}
	b.WriteString(")\n")
	// constants with several names per line
	fmt.Fprintf(&b, "const (\n\t%s_a, %s_b = iota, iota * 10\n\t%s_c, %s_d\n\t%s_e, %s_f\n)\n", base, base, base, base, base, base)
	g.top.WriteString(b.String())
	for _, i := range names {
		g.ln("fmt.Printf(\"%%v %%T\\n\", %s_%d, %s_%d)", base, i, base, i)
	}
	g.ln("fmt.Println(%s_a, %s_b, %s_c, %s_d, %s_e, %s_f)", base, base, base, base, base, base)
	// untyped constant arithmetic with big values
	big := g.topName("Big")
	k := g.between(60, 200)
	g.topf("const %s = 1 << %d\nconst %s_small = %s >> %d\nconst %s_f = %s / 3.0\nconst %s_r = 'a' + %d\nconst %s_s = \"con\" + \"cat\"", big, k, big, big, k-g.between(1, 30), big, big, big, g.between(0, 25), big)
	g.ln("fmt.Println(%s_small, %s_s, len(%s_s), %s_r, string(rune(%s_r)))", big, big, big, big, big)
	g.ln("fmt.Printf(\"%%T %%T %%T %%T\\n\", %s_small, %s_f > 1, %s_r, %s_s)", big, big, big, big)
	g.ln("fmt.Println(%s_f > 1e40, %s/%s, float64(%s_small))", big, big, big, big)
	// constants as array sizes, typed vs untyped
	g.topf("const %s_n = %d\ntype %s_arr [%s_n]int8", base, g.between(2, 5), base, base)
	g.ln("var arr %s_arr", base)
	g.ln("fmt.Println(len(arr), cap(arr), arr)")
	// a named type with typed constants and a String method
	nt := g.topName("Color")
	g.topf("type %s uint8\nconst (\n\t%s_Red %s = iota + 1\n\t%s_Green\n\t%s_Blue\n)\nfunc (c %s) String() string {\n\tswitch c {\n\tcase %s_Red:\n\t\treturn \"red\"\n\tcase %s_Green:\n\t\treturn \"green\"\n\t}\n\treturn \"color\" + fmt.Sprint(uint8(c))\n}", nt, nt, nt, nt, nt, nt, nt, nt)
	g.ln("fmt.Println(%s_Red, %s_Green, %s_Blue, %s(9))", nt, nt, nt, nt)
	g.ln("fmt.Printf(\"%%v %%d %%s %%x %%#v %%T\\n\", %s_Blue, %s_Blue, %s_Green, %s_Green, %s_Red, %s_Red)", nt, nt, nt, nt, nt, nt)
	g.ln("var c %s = 254", nt)
	g.ln("c += 3")
	g.ln("fmt.Println(c, uint8(c))")
	// local constants and shadowing of predeclared names
	g.ln("{")
	g.ln("\tconst len = 5")
	g.ln("\tconst true = false")
	g.ln("\tvar nil = 4")
	g.ln("\tfmt.Println(len, true, nil)")
	g.ln("}")
	g.ln("fmt.Println(len(\"abc\"), true)")
	g.stmts(g.between(0, 4), 1)
	g.showAll()
}

// ---- labels, goto, switch ----

func scenLabels(g *Gen) {
	n1, n2 := g.between(2, 5), g.between(2, 5)
	conds := []string{"(i*j)%%3 == 1", "(i+j)%%4 == 3", "i == j", "j > i+1", "(i^j)&1 == 1", "i*j > %d"}
	g.ln("outer:")
	g.enter(fmt.Sprintf("for i := 0; i < %d; i++", n1))
	g.enter(fmt.Sprintf("for j := 0; j < %d; j++", n2))
	c := pick(g, conds)
	if strings.Contains(c, "%d") {
		c = fmt.Sprintf(c, g.between(1, 8))
	}
	g.ln("if %s {", strings.ReplaceAll(c, "%%", "%"))
	g.ln("\tcontinue outer")
	g.ln("}")
	c2 := fmt.Sprintf(pick(g, []string{"i+j == %d", "i*j == %d", "j-i == %d"}), g.between(2, 7))
	g.ln("if %s {", c2)
	g.ln("\tbreak outer")
	g.ln("}")
	g.ln("fmt.Print(i, j, \" \")")
	g.leave()
	g.ln("fmt.Println(\"inner done\", i)")
	g.leave()
	g.ln("fmt.Println()")
	// break out of a switch inside a loop, and of a select-free labelled switch
	g.ln("sum := 0")
	g.ln("loop:")
	g.enter(fmt.Sprintf("for i := 0; i < %d; i++", g.between(4, 10)))
	g.enter("switch i % 4")
	g.ln("case 0:")
	g.ln("\tsum += 1")
	g.ln("\tfallthrough")
	g.ln("case 1:")
	g.ln("\tsum += 10")
	g.ln("case 2:")
	g.ln("\tif sum > %d {", g.between(10, 40))
	g.ln("\t\tbreak loop")
	g.ln("\t}")
	g.ln("\tbreak")
	g.ln("default:")
	g.ln("\tsum += 100")
	g.leave()
	g.ln("sum *= 2")
	g.leave()
	g.ln("fmt.Println(\"sum\", sum)")
	// goto: a loop built from labels, and a forward jump
	g.ln("k := 0")
	g.ln("again:")
	g.ln("if k < %d {", g.between(2, 6))
	g.ln("\tfmt.Print(\"g\", k, \" \")")
	g.ln("\tk++")
	g.ln("\tgoto again")
	g.ln("}")
	g.ln("if k > 100 {")
	g.ln("\tgoto skip")
	g.ln("}")
	g.ln("fmt.Println(\"not skipped\", k)")
	g.ln("skip:")
	g.ln("fmt.Println(\"after skip\")")
	// switch forms
	t := g.randIntType()
	x := g.declareVal("x", t)
	g.enter("switch y := " + x.Name + " % 3; y")
	g.ln("case 0, 1:")
	g.ln("\tfmt.Println(\"low\", y)")
	g.ln("\tfallthrough")
	g.ln("case 2:")
	g.ln("\tfmt.Println(\"two or fell\", y)")
	g.ln("default:")
	g.ln("\tfmt.Println(\"other\", y)")
	g.leave()
	g.enter("switch")
	g.ln("case %s > 0 && %s < 100:", x.Name, x.Name)
	g.ln("\tfmt.Println(\"small positive\")")
	g.ln("case %s < 0:", x.Name)
	g.ln("\tfmt.Println(\"negative\")")
	g.ln("default:")
	g.ln("\tfmt.Println(\"zero or large\")")
	g.leave()
	g.stmts(g.between(0, 5), 2)
	g.showAll()
}

// ---- multiple results and variadics ----

func scenMulti(g *Gen) {
	t1, t2 := g.randIntType(), g.randBasic()
	f := g.topName("mr")
	g.topFunc(fmt.Sprintf("%s(a %s, b %s) (%s, %s)", f, t1.Name, t2.Name, t2.Name, t1.Name), []*Var{{Name: "a", T: t1}, {Name: "b", T: t2}}, func() {
		r2 := g.expr(t1, 2)
		r1 := g.expr(t2, 2)
		g.sweep()
		g.ln("return %s, %s", r1, r2)
	})
	g.ln("x, y := %s(%s, %s)", f, g.lit(t1), g.expr(t2, 1))
	g.ln("fmt.Println(x, y)")
	// named results and a bare return, plus defer
	h := g.topName("nr")
	g.topFunc(fmt.Sprintf("%s(a %s) (r %s, ok bool)", h, t1.Name, t1.Name), []*Var{{Name: "a", T: t1}}, func() {
		g.ln("r = a")
		g.ln("if a > %s {", paren(g.intRaw(t1)))
		g.ln("\treturn")
		g.ln("}")
		g.ln("r = a * 2")
		g.ln("ok = true")
		g.ln("return")
	})
	g.ln("fmt.Println(%s(%s))", h, g.lit(t1))
	// spreading a call into an argument list
	sp := g.topName("sum2")
	g.topFunc(fmt.Sprintf("%s(x %s, y %s) string", sp, t2.Name, t1.Name), []*Var{{Name: "x", T: t2}, {Name: "y", T: t1}}, func() {
		g.ln("return fmt.Sprint(x, \"/\", y)")
	})
	g.ln("fmt.Println(%s(%s(%s, %s)))", sp, f, g.lit(t1), g.expr(t2, 1))
	// variadics
	vt := g.randIntType()
	v := g.topName("va")
	g.topFunc(fmt.Sprintf("%s(prefix string, xs ...%s) (n int, s %s)", v, vt.Name, vt.Name), []*Var{{Name: "prefix", T: tString}}, func() {
		g.ln("n = len(xs)")
		g.enter("for _, x := range xs")
		g.ln("s += x")
		g.leave()
		g.ln("fmt.Println(prefix, n, xs == nil, s)")
		g.ln("return")
	})
	g.ln("%s(\"none\")", v)
	g.ln("%s(\"one\", %s)", v, g.lit(vt))
	g.ln("%s(\"many\", %s, %s, %s)", v, g.lit(vt), g.lit(vt), g.lit(vt))
	g.ln("vs := []%s{%s, %s}", vt.Name, g.lit(vt), g.lit(vt))
	g.ln("n, s := %s(\"spread\", vs...)", v)
	g.ln("vs[0]++")
	g.ln("fmt.Println(n, s, vs)")
	g.ln("%s(\"nil\", []%s(nil)...)", v, vt.Name)
	// append of strings to bytes, copy from string
	g.ln("bb := append([]byte(\"ab\"), \"cd\"...)")
	g.ln("nb := copy(bb[1:], \"XYZ\")")
	g.ln("fmt.Println(string(bb), nb)")
	// anonymous functions returning several values, called immediately
	g.ln("q, r := func(a, b int) (int, int) { return a / b, a %% b }(%d, %d)", g.between(10, 99), g.between(1, 9))
	g.ln("fmt.Println(q, r)")
	g.stmts(g.between(0, 5), 1)
	g.showAll()
}

// ---- fmt on deep values ----

// leafKinds reports the kinds of the basic values inside t.
func leafKinds(t *Type, out map[Kind]bool) {
	switch t.K {
	case KInt, KFloat, KBool, KString:
		out[t.K] = true
	case KArray, KSlice:
		leafKinds(t.Elem, out)
	case KMap:
		leafKinds(t.Key, out)
		leafKinds(t.Elem, out)
	case KStruct:
		for _, f := range t.Fields {
			leafKinds(f.T, out)
		}
	case KPtr:
		leafKinds(t.Elem, out)
	}
}

func (g *Gen) verbDeep(t *Type) string {
	ks := map[Kind]bool{}
	leafKinds(t, ks)
	flags := pick(g, []string{"", "", "", "+", "#", "-", " ", "0", "5", "-8", "08", ".2", "6.3"})
	switch {
	case len(ks) == 1 && ks[KInt]:
		return "%" + flags + pick(g, []string{"d", "x", "X", "o", "b", "c", "q", "U", "v"})
	case len(ks) == 1 && ks[KString]:
		return "%" + flags + pick(g, []string{"s", "q", "x", "X", "v"})
	case len(ks) == 1 && ks[KBool]:
		return "%" + flags + pick(g, []string{"t", "v"})
	case len(ks) == 1 && ks[KFloat]:
		return "%" + flags + pick(g, []string{"v", "g", "f", "e"})
	}
	return "%" + pick(g, []string{"", "+", "#", "5", "-5", " "}) + "v"
}

func scenFmtDeep(g *Gen) {
	for i := g.between(3, 7); i > 0; i-- {
		t := g.randType(3)
		e := g.val(t, 2)
		v := g.declare("d", t, e)
		v.Used = true
		verb := g.verbDeep(t)
		switch g.roll(30, 30, 25, 15) {
		case 0:
			g.ln("fmt.Printf(%s, %s)", strconv.Quote(verb+"\n"), v.Name)
		case 1:
			g.ln("fmt.Printf(%s, %s, %s)", strconv.Quote(verb+" %T\n"), v.Name, v.Name)
		case 2:
			g.ln("fmt.Println(%s, %s)", v.Name, v.Name)
		default:
			g.ln("fmt.Printf(%s, %s)", strconv.Quote("%+v|%#v\n"), v.Name+", "+v.Name)
		}
		if g.chance(8) { // wrong verb or argument count: fmt reports it in the output
			g.ln("fmt.Printf(%s, %s)", strconv.Quote(pick(g, []string{"%d", "%s", "%!", "%z", "%d %d", "%[2]d", "%.*d", "%v%%"})+"\n"), v.Name)
		}
		if g.chance(6) {
			g.ln("fmt.Printf(%s)", strconv.Quote("no args %d %s\n"))
			g.ln("fmt.Println(fmt.Sprint(%s, %s), fmt.Sprintln(%s))", v.Name, v.Name, v.Name)
		}
	}
	g.ln("fmt.Printf(\"%%5.1f|%%-7.3e|%%+d|%%x|%%o|%%b|%%c|%%q|%%U|%%08.3f|%%6d|%%-6d|%%t\\n\", %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
		g.lit(tF64), g.lit(tF64), g.lit(intTs["int"]), g.lit(intTs["uint16"]), g.lit(intTs["int8"]), g.lit(intTs["uint8"]),
		g.lit(intTs["int32"]), g.lit(intTs["int32"]), g.lit(intTs["int32"]), g.lit(tF32), g.lit(intTs["int16"]), g.lit(intTs["int64"]), g.lit(tBool))
	g.ln("fmt.Printf(\"%%*d|%%-*d|%%.*f\\n\", 5, %s, 4, %s, 2, %s)", g.lit(intTs["int"]), g.lit(intTs["int"]), g.lit(tF64))
	g.ln("fmt.Printf(\"%%[2]d %%[1]d %%d\\n\", %s, %s)", g.lit(intTs["int"]), g.lit(intTs["int"]))
	g.ln("fmt.Printf(\"%%s %%v %%d\\n\", nil, nil, []any{nil, 1})")
	g.stmts(g.between(0, 4), 1)
}
