package gen

import (
	"fmt"
	"strconv"
	"strings"
)

func init() {
	register("ifaces", 12, []string{"ifaces", "methods"}, scenIfaces)
	register("embed", 9, []string{"embed", "methods"}, scenEmbed)
	register("methodvals", 7, []string{"methodvals", "methods"}, scenMethodVals)
	register("typeswitch", 8, []string{"typeswitch"}, scenTypeSwitch)
	register("generics", 9, []string{"generics"}, scenGenerics)
	register("methods", 8, []string{"methods"}, scenMethods)
}

// ---- a world of interfaces and implementations ----

type mdecl struct {
	name   string
	params []*Type
	res    *Type
}

type impl struct {
	name   string // type name
	ptr    bool   // methods have pointer receivers
	fields []Field
	basic  *Type // a defined basic type instead of a struct
	T      *Type
}

func (g *Gen) simpleType() *Type {
	switch g.roll(60, 25, 15) {
	case 0:
		return g.randIntType()
	case 1:
		return tString
	}
	return tBool
}

// ifaceWorld declares an interface and several types that implement it.
type world struct {
	iname string
	meths []mdecl
	impls []*impl
}

func (g *Gen) newWorld(nimpl int) *world {
	w := &world{iname: g.topName("I")}
	n := g.between(1, 3)
	names := []string{"Area", "Scale", "Tag", "Combine", "Score", "Step"}
	g.shuffle(names)
	for i := 0; i < n; i++ {
		m := mdecl{name: names[i], res: g.simpleType()}
		for j := g.between(0, 2); j > 0; j-- {
			m.params = append(m.params, g.simpleType())
		}
		w.meths = append(w.meths, m)
	}
	// every implementation also has Name, which is nil-receiver safe
	var sb strings.Builder
	fmt.Fprintf(&sb, "type %s interface {\n\tName() string\n", w.iname)
	for _, m := range w.meths {
		fmt.Fprintf(&sb, "\t%s(%s) %s\n", m.name, typeList(m.params), m.res.Name)
	}
	sb.WriteString("}\n")
	g.top.WriteString(sb.String())
	for i := 0; i < nimpl; i++ {
		w.impls = append(w.impls, g.newImpl(w, i))
	}
	return w
}

func typeList(ts []*Type) string {
	var ss []string
	for _, t := range ts {
		ss = append(ss, t.Name)
	}
	return strings.Join(ss, ", ")
}

func (g *Gen) newImpl(w *world, idx int) *impl {
	im := &impl{name: g.topName("C"), ptr: g.chance(40)}
	var recvT *Type
	if g.chance(25) {
		im.basic = intTs[pick(g, []string{"int", "int8", "uint8", "int16", "uint32", "int64"})]
		recvT = g.newNamedBasic(im.basic)
		im.name = recvT.Name
		im.T = recvT
		im.ptr = false
	} else {
		var fs []Field
		names := []string{"a", "b", "c", "d"}
		g.shuffle(names)
		for i := g.between(1, 3); i > 0; i-- {
			fs = append(fs, Field{Name: names[i-1], T: g.simpleType()})
		}
		im.fields = fs
		t := &Type{K: KStruct, Name: im.name, Named: true, NoPtr: true, Fields: fs}
		var sb strings.Builder
		fmt.Fprintf(&sb, "type %s struct {\n", im.name)
		for _, f := range fs {
			fmt.Fprintf(&sb, "\t%s %s\n", f.Name, f.T.Name)
		}
		sb.WriteString("}\n")
		g.top.WriteString(sb.String())
		im.T = t
		recvT = t
	}
	recvDecl := "r " + im.name
	if im.ptr {
		recvDecl = "r *" + im.name
	}
	// the receiver as a variable that expressions can read
	mkRecv := func() *Var {
		if im.basic != nil {
			g.ln("x := %s(r)", im.basic.Name)
			return &Var{Name: "x", T: im.basic}
		}
		if im.ptr {
			return &Var{Name: "r", T: ptrTo(recvT), NonNil: true}
		}
		return &Var{Name: "r", T: recvT}
	}
	// Name
	g.topFunc(fmt.Sprintf("(%s) Name() string", recvDecl), nil, func() {
		if im.ptr {
			g.ln("if r == nil {")
			g.ln("\treturn %q", im.name+"(nil)")
			g.ln("}")
		}
		g.ln("return %q", im.name)
	})
	for _, m := range w.meths {
		var ps []*Var
		var pd []string
		for j, pt := range m.params {
			nm := fmt.Sprintf("p%d", j)
			ps = append(ps, &Var{Name: nm, T: pt})
			pd = append(pd, nm+" "+pt.Name)
		}
		g.topFunc(fmt.Sprintf("(%s) %s(%s) %s", recvDecl, m.name, strings.Join(pd, ", "), m.res.Name), ps, func() {
			rv := mkRecv()
			rv.Used = im.basic == nil
			rv.Param = im.basic == nil
			g.add(rv)
			if g.chance(40) {
				g.ln("fmt.Println(%q, %s)", im.name+"."+m.name, g.tracePath(rv))
			}
			if g.chance(30) && im.ptr {
				// a pointer receiver mutates the value
				var basics []Path
				g.walk(rv, rv.Name, rv.T, 0, &basics)
				for _, p := range basics {
					if p.T.K == KInt && p.E != rv.Name {
						g.ln("%s++", p.E)
						break
					}
				}
			}
			e := g.expr(m.res, 3)
			g.sweep()
			g.ln("return %s", e)
		})
	}
	// optional String and Error methods
	if g.chance(30) && idx%3 == 0 {
		g.topFunc(fmt.Sprintf("(%s) String() string", recvDecl), nil, func() {
			if im.ptr {
				g.ln("if r == nil {")
				g.ln("\treturn \"<nilstr>\"")
				g.ln("}")
			}
			if im.basic != nil {
				g.ln("return fmt.Sprintf(\"%s<%%d>\", %s(r))", im.name, im.basic.Name)
			} else {
				var fa []string
				for _, f := range im.fields {
					fa = append(fa, "r."+f.Name)
				}
				g.ln("return fmt.Sprint(%q, %s)", im.name+"<", strings.Join(fa, ", \"|\", "))
			}
		})
	}
	return im
}

// tracePath returns an expression of the receiver that prints without addresses.
func (g *Gen) tracePath(rv *Var) string {
	if rv.T.K == KPtr {
		return "*" + rv.Name
	}
	return rv.Name
}

func (g *Gen) implValue(im *impl, asIface bool) string {
	if im.basic != nil {
		return fmt.Sprintf("%s(%s)", im.name, g.intRaw(im.basic))
	}
	parts := make([]string, len(im.fields))
	for i, f := range im.fields {
		parts[i] = g.lit(f.T)
	}
	v := im.name + "{" + strings.Join(parts, ", ") + "}"
	if im.ptr {
		return "&" + v
	}
	return v
}

func scenIfaces(g *Gen) {
	w := g.newWorld(g.between(2, 4))
	var vals []string
	for i := 0; i < g.between(3, 6); i++ {
		vals = append(vals, g.implValue(pick(g, w.impls), true))
	}
	g.ln("vals := []%s{%s}", w.iname, strings.Join(vals, ", "))
	g.enter("for i, v := range vals")
	g.ln("fmt.Println(i, v.Name())")
	for _, m := range w.meths {
		args := make([]string, len(m.params))
		for j, p := range m.params {
			args[j] = g.lit(p)
		}
		g.ln("fmt.Println(v.%s(%s))", m.name, strings.Join(args, ", "))
	}
	g.ln("fmt.Println(v)")
	g.ln("fmt.Printf(\"%%v|%%+v|%%T|%%s\\n\", v, v, v, v)")
	g.leave()
	// equality of interface values and use as map keys
	g.ln("same := map[%s]int{}", w.iname)
	g.enter("for _, v := range vals")
	g.ln("same[v]++")
	g.leave()
	g.ln("fmt.Println(len(same), len(vals))")
	// type assertions
	for _, im := range w.impls {
		g.ln("if c, ok := vals[%d].(%s); ok {", g.rn(len(vals)), im.typeStr())
		g.ln("\tfmt.Println(\"is %s\", c.Name())", im.name)
		g.ln("} else {")
		g.ln("\tfmt.Println(\"not %s\", ok)", im.name)
		g.ln("}")
	}
	// the typed nil: an interface holding a nil pointer is not nil
	for _, im := range w.impls {
		if im.ptr {
			g.ln("var np *%s", im.name)
			g.ln("var ni %s = np", w.iname)
			g.ln("var zi %s", w.iname)
			g.ln("fmt.Println(ni == nil, np == nil, zi == nil, ni != nil, ni.Name())")
			g.ln("fmt.Printf(\"%%T %%T %%v\\n\", ni, zi, zi)")
			break
		}
	}
	// a smaller interface satisfied by all
	g.ln("var nm interface{ Name() string } = vals[0]")
	g.ln("fmt.Println(nm.Name())")
	g.ln("if _, ok := nm.(%s); ok {", w.iname)
	g.ln("\tfmt.Println(\"has all methods\")")
	g.ln("}")
	// any holding each kind of value, compared
	g.ln("var a1, a2 any = vals[0], vals[0]")
	g.ln("fmt.Println(a1 == a2, a1 == any(nil), a1 != a2)")
	// an interface slice mutated in place through pointer elements
	g.stmts(g.between(0, 4), 1)
	g.showAll()
}

func (im *impl) typeStr() string {
	if im.ptr {
		return "*" + im.name
	}
	return im.name
}

// ---- type switches ----

func scenTypeSwitch(g *Gen) {
	// a list of values of many dynamic types
	nt := g.newNamedBasic(g.randIntType())
	st := g.newStruct(1)
	g.topf("func (n %s) String() string { return \"N\" + fmt.Sprint(%s(n)) }", nt.Name, nt.Under)
	// the error type
	en := g.topName("MyErr")
	g.topf("type %s struct{ code int }\nfunc (e %s) Error() string { return fmt.Sprint(\"err\", e.code) }", en, en)
	vals := []string{
		g.lit(intTs["int"]), g.lit(intTs["int8"]), g.lit(intTs["uint8"]), g.lit(intTs["int32"]), g.lit(intTs["uint64"]),
		g.lit(tF64), g.lit(tF32), g.strRaw(), g.lit(tBool), "nil",
		nt.Name + "(" + g.intRaw(nt) + ")", g.val(st, 1), "&" + g.ctor(st, 1),
		"[]int8{1, 2}", "[2]int8{3, 4}", "map[string]int{\"a\": 1}", en + "{7}", "error(" + en + "{8})",
		"func() {}", "[]any{1, \"two\"}", "'x'", "struct{}{}", "uint(3)", "int16(-5)",
	}
	g.shuffle(vals)
	vals = vals[:g.between(8, len(vals))]
	g.ln("vals := []any{%s}", strings.Join(vals, ", "))
	g.enter("for i, v := range vals")
	g.enter("switch x := v.(type)")
	g.ln("case nil:")
	g.ln("\tfmt.Println(i, \"nil\", x)")
	g.ln("case int, int8:")
	g.ln("\tfmt.Printf(\"%%d small-int %%v %%T\\n\", i, x, x)")
	g.ln("case uint8, uint16, uint32:")
	g.ln("\tfmt.Printf(\"%%d unsigned %%v %%T\\n\", i, x, x)")
	g.ln("case int32:")
	g.ln("\tfmt.Println(i, \"rune-or-int32\", x+1)")
	g.ln("case float64:")
	g.ln("\tfmt.Println(i, \"f64\", x*2)")
	g.ln("case float32:")
	g.ln("\tfmt.Println(i, \"f32\", x/2)")
	g.ln("case string:")
	g.ln("\tfmt.Println(i, \"string\", len(x), x)")
	g.ln("case bool:")
	g.ln("\tfmt.Println(i, \"bool\", !x)")
	g.ln("case %s:", nt.Name)
	g.ln("\tfmt.Println(i, \"named\", x, uint64(x))")
	g.ln("case %s:", st.Name)
	g.ln("\tfmt.Printf(\"%%d struct %%+v\\n\", i, x)")
	g.ln("case *%s:", st.Name)
	g.ln("\tfmt.Printf(\"%%d struct-ptr %%+v\\n\", i, *x)")
	g.ln("case []int8:")
	g.ln("\tfmt.Println(i, \"slice\", len(x), x)")
	g.ln("case [2]int8:")
	g.ln("\tfmt.Println(i, \"array\", x)")
	g.ln("case map[string]int:")
	g.ln("\tfmt.Println(i, \"map\", x)")
	g.ln("case error:")
	g.ln("\tfmt.Println(i, \"error\", x.Error(), x)")
	g.ln("case fmt.Stringer:")
	g.ln("\tfmt.Println(i, \"stringer\", x.String())")
	g.ln("case func():")
	g.ln("\tfmt.Println(i, \"func\", x != nil)")
	g.ln("case []any:")
	g.ln("\tfmt.Println(i, \"anys\", len(x), x)")
	g.ln("default:")
	g.ln("\tfmt.Printf(\"%%d other %%T\\n\", i, x)") // not %v: x may be a func, whose address differs
	g.leave()
	g.leave()
	// assertions with comma-ok on a few
	g.enter("for i, v := range vals")
	g.ln("n, ok := v.(int)")
	g.ln("s, ok2 := v.(string)")
	g.ln("e, ok3 := v.(error)")
	g.ln("_, ok4 := v.(fmt.Stringer)")
	g.ln("fmt.Println(i, n, ok, s, ok2, e == nil, ok3, ok4)")
	g.leave()
	// a type switch on an interface with a shared case and a no-binding form
	g.enter("switch v := vals[0].(type)")
	g.ln("case int8, int16:")
	g.ln("\tfmt.Printf(\"%%T\\n\", v)")
	g.ln("case nil:")
	g.ln("\tfmt.Println(\"nil\")")
	g.ln("default:")
	g.ln("\tfmt.Printf(\"%%T\\n\", v)")
	g.leave()
	g.enter("switch vals[0].(type)")
	g.ln("case string, bool:")
	g.ln("\tfmt.Println(\"stringish\")")
	g.ln("default:")
	g.ln("\tfmt.Println(\"other\")")
	g.leave()
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

// ---- methods on structs and named types ----

func scenMethods(g *Gen) {
	st := g.newStructWith([]Field{{Name: "A", T: g.simpleType()}, {Name: "B", T: intTs["int16"]}, {Name: "C", T: tString}})
	nt := g.newNamedBasic(intTs[pick(g, []string{"int8", "uint8", "int32", "uint16"})])
	// value and pointer receivers
	g.topFunc(fmt.Sprintf("(s %s) Sum() int16", st.Name), nil, func() { g.ln("return s.B + int16(len(s.C))") })
	g.topFunc(fmt.Sprintf("(s *%s) Bump(by int16) *%s", st.Name, st.Name), nil, func() {
		g.ln("s.B += by")
		g.ln("s.C += \"!\"")
		g.ln("return s")
	})
	g.topFunc(fmt.Sprintf("(s %s) Mutate() %s", st.Name, st.Name), nil, func() {
		g.ln("s.B = 99")
		g.ln("return s")
	})
	g.topFunc(fmt.Sprintf("(n %s) Double() %s", nt.Name, nt.Name), nil, func() { g.ln("return n * 2") })
	g.topFunc(fmt.Sprintf("(n *%s) Inc()", nt.Name), nil, func() { g.ln("*n++") })
	v := g.declareVal("s", st)
	g.ln("fmt.Println(%s.Sum())", v.Name)
	g.ln("%s.Bump(3).Bump(4)", v.Name) // chained pointer methods on an addressable value
	g.ln("fmt.Println(%s, %s.Sum())", v.Name, v.Name)
	g.ln("m := %s.Mutate()", v.Name)
	g.ln("fmt.Println(m.B, %s.B)", v.Name)
	p := g.declare("p", ptrTo(st), "&"+v.Name)
	g.ln("fmt.Println(%s.Sum(), (*%s).Sum())", p.Name, p.Name)
	g.ln("m2 := %s.Mutate()", p.Name)
	g.ln("m2.Bump(1).Bump(1)")
	g.ln("fmt.Println(m2.B)")
	g.ln("fmt.Println(*%s)", p.Name)
	n := g.declareVal("n", nt)
	g.ln("%s.Inc()", n.Name)
	g.ln("%s.Inc()", n.Name)
	g.ln("fmt.Println(%s, %s.Double(), %s.Double().Double())", n.Name, n.Name, n.Name)
	// methods on slice and map element addresses
	g.ln("ns := []%s{1, 2, 3}", nt.Name)
	g.ln("for i := range ns { ns[i].Inc() }")
	g.ln("fmt.Println(ns)")
	g.ln("arr := [2]%s{{B: 4}, {B: 5}}", st.Name)
	g.ln("arr[1].Bump(10)")
	g.ln("fmt.Println(arr[1].B, arr[0].B)")
	// a value receiver gets a copy, also through a range variable
	g.ln("ss := []%s{{B: 1}, {B: 2}}", st.Name)
	g.enter("for _, e := range ss")
	g.ln("e.Bump(5)")
	g.leave()
	g.ln("fmt.Println(ss[0].B, ss[1].B)")
	g.enter("for i := range ss")
	g.ln("ss[i].Bump(5)")
	g.leave()
	g.ln("fmt.Println(ss[0].B, ss[1].B)")
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

// ---- method values and method expressions ----

func scenMethodVals(g *Gen) {
	st := g.newStructWith([]Field{{Name: "N", T: intTs["int8"]}, {Name: "S", T: tString}})
	g.topFunc(fmt.Sprintf("(t %s) Show(tag string) string", st.Name), nil, func() {
		g.ln("return fmt.Sprint(tag, \":\", t.N, t.S)")
	})
	g.topFunc(fmt.Sprintf("(t *%s) Add(d int8) int8", st.Name), nil, func() {
		g.ln("t.N += d")
		g.ln("return t.N")
	})
	g.ln("t := %s{N: %s, S: %s}", st.Name, g.lit(intTs["int8"]), g.strRaw())
	// a method value binds a copy of a value receiver, and the pointer for a pointer receiver
	g.ln("show := t.Show")
	g.ln("add := t.Add")
	g.ln("t.N = 100")
	g.ln("t.S = \"changed\"")
	g.ln("fmt.Println(show(\"bound\"), t.Show(\"live\"))")
	g.ln("fmt.Println(add(5), t.N, add(120), t.N)")
	// method expressions
	g.ln("f1 := %s.Show", st.Name)
	g.ln("f2 := (*%s).Add", st.Name)
	g.ln("f3 := (*%s).Show", st.Name)
	g.ln("fmt.Println(f1(t, \"expr\"), f2(&t, 1), f3(&t, \"ptr\"))")
	// method values from an interface
	g.ln("var sh interface{ Show(string) string } = t")
	g.ln("g1 := sh.Show")
	g.ln("t.N = 7")
	g.ln("fmt.Println(g1(\"iface\"))")
	g.ln("var sp interface{ Add(int8) int8; Show(string) string } = &t")
	g.ln("g2 := sp.Add")
	g.ln("fmt.Println(g2(1), g2(1), t.N)")
	// defers and goroutine-free calls through method values
	g.ln("fs := []func(string) string{t.Show, (&t).Show, show}")
	g.ln("t.N = -1")
	g.enter("for _, f := range fs")
	g.ln("fmt.Println(f(\"slice\"))")
	g.leave()
	// func values in maps and struct fields
	g.ln("ops := map[string]func(int8) int8{\"add\": t.Add, \"neg\": func(x int8) int8 { return -x }}")
	g.ln("fmt.Println(ops[\"neg\"](5), ops[\"add\"](2), t.N, ops[\"none\"] == nil)")
	g.ln("holder := struct{ cb func(string) string }{t.Show}")
	g.ln("t.S = \"later\"")
	g.ln("fmt.Println(holder.cb(\"field\"))")
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

// ---- embedding ----

func scenEmbed(g *Gen) {
	base := g.topName("Base")
	mid := g.topName("Mid")
	top := g.topName("Top")
	g.topf("type %s struct {\n\tID int8\n\tName string\n}", base)
	g.topf("func (b %s) Describe() string { return fmt.Sprint(\"base \", b.ID, \" \", b.Name) }", base)
	g.topf("func (b *%s) Rename(s string) { b.Name = s }", base)
	g.topf("func (b %s) Hello() string { return \"hello from base \" + b.Name }", base)
	g.topf("func (b %s) Who() string { return \"base\" }", base)
	shadowField := g.chance(50)
	g.topf("type %s struct {\n\t%s\n\tLevel uint8", mid, base)
	if shadowField {
		g.topf("\tName string")
	}
	g.topf("}")
	g.topf("func (m %s) Hello() string { return \"mid(\" + m.%s.Hello() + \")\" }", mid, base)
	g.topf("func (m *%s) Up() { m.Level++ }", mid)
	ptrEmbed := g.chance(60)
	if ptrEmbed {
		g.topf("type %s struct {\n\t*%s\n\tExtra int16\n}", top, mid)
	} else {
		g.topf("type %s struct {\n\t%s\n\tExtra int16\n}", top, mid)
	}
	g.topf("func (t %s) Who() string { return \"top>\" + t.%s.Who() }", top, mid)
	// an interface satisfied through promotion
	desc := g.topName("Describer")
	g.topf("type %s interface {\n\tDescribe() string\n\tHello() string\n}", desc)
	// a struct embedding an interface
	svc := g.topName("Svc")
	g.topf("type %s struct {\n\t%s\n\tCalls int\n}", svc, desc)
	g.topf("func (s *%s) Describe() string { s.Calls++; return \"svc/\" + s.%s.Describe() }", svc, desc)
	mk := fmt.Sprintf("%s{ID: %s, Name: %s}", base, g.lit(intTs["int8"]), g.strRaw())
	if ptrEmbed {
		g.ln("t := %s{%s: &%s{%s: %s, Level: %d}, Extra: 5}", top, mid, mid, base, mk, g.between(0, 250))
	} else {
		g.ln("t := %s{%s: %s{%s: %s, Level: %d}, Extra: 5}", top, mid, mid, base, mk, g.between(0, 250))
	}
	g.ln("fmt.Println(t.ID, t.Name, t.Level, t.Extra, t.Describe(), t.Hello(), t.Who())")
	g.ln("fmt.Println(t.%s.%s.Name, t.%s.%s.Hello())", mid, base, mid, base)
	g.ln("t.Rename(%s)", g.strRaw())
	g.ln("t.Up()")
	g.ln("t.ID -= 3")
	g.ln("fmt.Println(t.ID, t.Name, t.Level, t.Describe())")
	// copies: pointer embedding shares, value embedding copies
	g.ln("c := t")
	g.ln("c.Rename(\"copy\")")
	g.ln("c.Up()")
	g.ln("c.Extra = 9")
	g.ln("fmt.Println(t.Name, t.Level, t.Extra, \"|\", c.Name, c.Level, c.Extra)")
	// through the interface
	g.ln("var d %s = t", desc)
	g.ln("fmt.Println(d.Describe(), d.Hello())")
	g.ln("var d2 %s = &t", desc)
	g.ln("t.Rename(\"renamed\")")
	g.ln("fmt.Println(d.Describe() == d2.Describe(), d2.Hello())")
	// embedded interface in a struct, promoted calls
	g.ln("s := &%s{%s: d}", svc, desc)
	g.ln("fmt.Println(s.Describe(), s.Hello(), s.Calls)")
	g.ln("var sd %s = s", desc)
	g.ln("fmt.Println(sd.Describe(), s.Calls)")
	// the method sets: only an addressable value can call pointer methods
	g.ln("m := t.%s", mid)
	if ptrEmbed {
		g.ln("m.Up()")
		g.ln("fmt.Println(m.Level, t.Level)")
	} else {
		g.ln("m.Up()")
		g.ln("fmt.Println(m.Level, t.Level)")
	}
	g.ln("fmt.Printf(\"%%+v\\n\", t.%s.%s)", mid, base)
	g.ln("fmt.Printf(\"%%v %%T\\n\", t.Extra, t.Hello)")
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

// ---- generic functions ----

const genericPrelude = `type %[1]sInteger interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type %[1]sNumber interface {
	%[1]sInteger | ~float32 | ~float64
}

type %[1]sOrdered interface {
	%[1]sNumber | ~string
}
`

var genericFuncs = []struct {
	name, src string
}{
	{"Sum", `func %[1]sSum[T %[1]sNumber](xs []T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}`},
	{"Max", `func %[1]sMax[T %[1]sOrdered](a, b T) T {
	if a > b {
		return a
	}
	return b
}`},
	{"Map", `func %[1]sMap[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}`},
	{"Filter", `func %[1]sFilter[T any](xs []T, f func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if f(x) {
			out = append(out, x)
		}
	}
	return out
}`},
	{"Reduce", `func %[1]sReduce[T, A any](xs []T, init A, f func(A, T) A) A {
	acc := init
	for _, x := range xs {
		acc = f(acc, x)
	}
	return acc
}`},
	{"Index", `func %[1]sIndex[T comparable](xs []T, v T) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}`},
	{"Reverse", `func %[1]sReverse[T any](xs []T) {
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
}`},
	{"Swap", `func %[1]sSwap[A, B any](a A, b B) (B, A) {
	return b, a
}`},
	{"Zero", `func %[1]sZero[T any]() T {
	var z T
	return z
}`},
	{"Ptr", `func %[1]sPtr[T any](v T) *T {
	return &v
}`},
	{"Clamp", `func %[1]sClamp[T %[1]sOrdered](v, lo, hi T) T {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}`},
	{"Wrap", `func %[1]sWrap[T %[1]sInteger](a T) T {
	a++
	a *= a
	return a ^ (a >> 1)
}`},
	{"Shl", `func %[1]sShl[T %[1]sInteger](a T, n uint) T {
	return a<<n | a>>(n+1)
}`},
	{"Div", `func %[1]sDiv[T %[1]sInteger](a, b T) (T, T) {
	return a / (b | 1), a %% (b | 1)
}`},
	{"Neg", `func %[1]sNeg[T %[1]sNumber](a T) T {
	return -a
}`},
	{"Apply", `func %[1]sApply[T any](v T, fs ...func(T) T) T {
	for _, f := range fs {
		v = f(v)
	}
	return v
}`},
	{"Count", `func %[1]sCount[T comparable](xs []T, v T) (n int) {
	for _, x := range xs {
		if x == v {
			n++
		}
	}
	return
}`},
	{"Fill", `func %[1]sFill[T any](n int, v T) []T {
	s := make([]T, n)
	for i := range s {
		s[i] = v
	}
	return s
}`},
	{"Describe", `func %[1]sDescribe[T any](v T) string {
	return fmt.Sprintf("%%T=%%v", v, v)
}`},
}

func scenGenerics(g *Gen) {
	pfx := g.pfx + "g" + strconv.Itoa(g.nid) + "_"
	g.nid++
	g.topf(genericPrelude, pfx)
	// the type arguments: basic and defined types
	nt := g.newNamedBasic(g.randIntType())
	mine := map[string]bool{}
	for i := 0; i < g.between(4, 8); i++ {
		f := pick(g, genericFuncs)
		if mine[f.name] {
			continue
		}
		mine[f.name] = true
		g.topf(f.src, pfx)
		g.topf("")
	}
	it := func() *Type { return g.randIntType() }
	if mine["Sum"] {
		t := it()
		g.ln("fmt.Println(%sSum(%s), %sSum[%s](%s), %sSum(%s))", pfx, lits2(g, t, 4), pfx, nt.Name, "[]"+nt.Name+"{"+g.intRaw(nt)+", "+g.intRaw(nt)+"}", pfx, "[]float64{"+g.floatRaw()+", "+g.floatRaw()+"}")
	}
	if mine["Max"] {
		t := it()
		g.ln("fmt.Println(%sMax(%s, %s), %sMax(%s, %s), %sMax(%s, %s), %sMax[%s](%s, %s))", pfx, g.lit(t), g.lit(t), pfx, g.strRaw(), g.strRaw(), pfx, g.lit(tF64), g.lit(tF64), pfx, nt.Name, g.intRaw(nt), g.intRaw(nt))
	}
	if mine["Map"] {
		t := it()
		g.ln("fmt.Println(%sMap(%s, func(x %s) string { return fmt.Sprint(x * 2) }), %sMap([]string{\"a\", \"bb\"}, func(s string) int { return len(s) }))", pfx, lits2(g, t, 3), t.Name, pfx)
	}
	if mine["Filter"] {
		t := it()
		g.ln("fmt.Println(%sFilter(%s, func(x %s) bool { return x%%2 == 0 || x > 50 }), %sFilter([]string{\"a\", \"\", \"b\"}, func(s string) bool { return s != \"\" }))", pfx, lits2(g, t, 6), t.Name, pfx)
	}
	if mine["Reduce"] {
		t := it()
		g.ln("fmt.Println(%sReduce(%s, \"\", func(a string, x %s) string { return a + fmt.Sprint(x) + \",\" }), %sReduce(%s, %s(0), func(a %s, x %s) %s { return a*3 + x }))", pfx, lits2(g, t, 4), t.Name, pfx, lits2(g, t, 4), t.Name, t.Name, t.Name, t.Name)
	}
	if mine["Index"] {
		g.ln("fmt.Println(%sIndex([]string{\"x\", \"y\", \"z\"}, \"z\"), %sIndex([]int8{1, 2}, 3), %sIndex([]%s{1, 2}, 2))", pfx, pfx, pfx, nt.Name)
	}
	if mine["Reverse"] {
		t := it()
		g.ln("rv := %s", lits2(g, t, 5))
		g.ln("%sReverse(rv)", pfx)
		g.ln("fmt.Println(rv)")
		g.ln("rs := []string{\"a\", \"b\", \"c\"}")
		g.ln("%sReverse(rs)", pfx)
		g.ln("fmt.Println(rs)")
	}
	if mine["Swap"] {
		g.ln("fmt.Println(%sSwap(%s, %s))", pfx, g.lit(it()), g.strRaw())
		g.ln("a1, b1 := %sSwap[int8, bool](1, true)", pfx)
		g.ln("fmt.Println(a1, b1)")
	}
	if mine["Zero"] {
		g.ln("fmt.Printf(\"%%v %%q %%v %%v\\n\", %sZero[int8](), %sZero[string](), %sZero[[]int](), %sZero[*int]() == nil)", pfx, pfx, pfx, pfx)
	}
	if mine["Ptr"] {
		g.ln("pp := %sPtr(%s)", pfx, g.lit(intTs["int16"]))
		g.ln("*pp += 5")
		g.ln("fmt.Println(*pp, *%sPtr(\"s\"), %sPtr(1) == %sPtr(1))", pfx, pfx, pfx)
	}
	if mine["Clamp"] {
		t := it()
		g.ln("fmt.Println(%sClamp(%s, %s, %s), %sClamp(%s, \"b\", \"d\"))", pfx, g.lit(t), g.lit(t), g.lit(t), pfx, g.strRaw())
	}
	for _, f := range []string{"Wrap", "Neg"} {
		if mine[f] {
			for i := 0; i < g.between(1, 3); i++ {
				t := it()
				g.ln("fmt.Println(%s%s(%s))", pfx, f, g.lit(t))
			}
			if f == "Neg" {
				g.ln("fmt.Println(%sNeg(%s), %sNeg(%s))", pfx, g.lit(tF32), pfx, g.lit(tF64))
			}
		}
	}
	if mine["Shl"] {
		t := it()
		g.ln("fmt.Println(%sShl(%s, %d), %sShl(%s, %d))", pfx, g.lit(t), g.between(0, 70), pfx, g.lit(t), g.between(0, 9))
	}
	if mine["Div"] {
		t := it()
		g.ln("fmt.Println(%sDiv(%s, %s))", pfx, g.lit(t), g.lit(t))
	}
	if mine["Apply"] {
		t := it()
		g.ln("fmt.Println(%sApply(%s, func(x %s) %s { return x * 2 }, func(x %s) %s { return x + 1 }), %sApply(\"s\"))", pfx, g.lit(t), t.Name, t.Name, t.Name, t.Name, pfx)
	}
	if mine["Count"] {
		g.ln("fmt.Println(%sCount([]int8{1, 1, 2}, 1), %sCount([]string{\"a\"}, \"b\"))", pfx, pfx)
	}
	if mine["Fill"] {
		g.ln("fmt.Println(%sFill(3, %s), %sFill(2, \"x\"), len(%sFill(0, 1.5)))", pfx, g.lit(it()), pfx, pfx)
	}
	if mine["Describe"] {
		g.ln("fmt.Println(%sDescribe(%s), %sDescribe(%s), %sDescribe([]int{1}), %sDescribe(%s{N: 1}))", pfx, g.lit(it()), pfx, g.strRaw(), pfx, pfx, g.dummyStruct())
	}
	// instantiated function values
	if mine["Sum"] {
		g.ln("sf := %sSum[int16]", pfx)
		g.ln("fmt.Println(sf([]int16{30000, 30000}))")
	}
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

func lits2(g *Gen, t *Type, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = g.lit(t)
	}
	return "[]" + t.Name + "{" + strings.Join(parts, ", ") + "}"
}

func (g *Gen) dummyStruct() string {
	name := g.topName("D")
	g.topf("type %s struct{ N int8 }", name)
	return name
}
