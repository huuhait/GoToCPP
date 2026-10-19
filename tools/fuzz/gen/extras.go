package gen

import (
	"fmt"
	"strings"
)

// The constructs of Options.Extras (step 15.8, second round): the shapes that the Go test suite's
// bug cluster of the 15.10 conformance run is made of, and that the first generator never wrote:
// multi-value call forwarding f(g()) (to functions, variadic functions, methods, closures, defer,
// go, fmt.Println and return), constants with iota (untyped and of named types, with shifts),
// shifts with constant and variable counts, rune literals and conversions, three-index slices,
// named slice, map and element types with conversions to and from string, method expressions,
// anonymous structs, struct tags and goto. Every statement here is valid by construction in the
// same way as the rest of the generator (see the package comment), and the programs stay
// deterministic: the choices come from the Rand only.

// A cgroup is a group of constants of one type: the key is the type's String.
type cgroup struct {
	key   string
	names []string
}

// A fwdPair is a function whose results are the parameters of another one.
type fwdPair struct {
	src, sink *fn
}

// A fwdMeth is a method that takes the results of a function.
type fwdMeth struct {
	recv *typ
	m    *method
	src  *fn
}

// mybTypes are the names of the declared named slice, map and element types.
type mybTypes struct {
	elem, bytes, runes, mp string
}

// newForwarders makes the functions of the multi-value forwarding: for a list of types ts, a
// function with the results ts, a function (and a method) with the parameters ts, a function that
// returns the call (return f()), and for the variadic case a function with the results (a, b, b)
// and one with the parameters (a, ...b).
func (g *gen) newForwarders() {
	for i := 1 + g.intn(2); i > 0; i-- {
		var ts []*typ
		for j := 2 + g.intn(2); j > 0; j-- {
			ts = append(ts, g.valueType(1, nil))
		}
		mv := &fn{name: g.name("f"), results: ts}
		if g.chance(2) {
			mv.params = []*typ{g.valueType(1, nil)}
		}
		sink := &fn{name: g.name("f"), params: ts}
		if g.chance(2) {
			sink.results = []*typ{g.valueType(1, nil)}
		}
		mv2 := &fn{name: g.name("f"), results: []*typ{ts[0], ts[1], ts[1]}}
		vs := &fn{name: g.name("f"), params: []*typ{ts[0], {kind: kSlice, elem: ts[1]}}, variadic: true}
		ret := &fn{name: g.name("f"), results: ts, fwd: mv}
		g.funcs = append(g.funcs, mv, sink, mv2, vs, ret)
		g.fwds = append(g.fwds, fwdPair{mv, sink}, fwdPair{mv2, vs})
		if len(g.structs) > 0 {
			s := g.structs[g.intn(len(g.structs))]
			m := &method{name: g.name("m"), ptr: g.chance(2), params: ts}
			s.methods = append(s.methods, m)
			g.fwdMeths = append(g.fwdMeths, fwdMeth{s, m, mv})
		}
	}
}

// newConsts makes the constant declarations (g.constDecl) and records the constants, by type.
func (g *gen) newConsts() {
	add := func(key string, names ...string) {
		g.consts[key] = append(g.consts[key], names...)
	}
	b := &g.constDecl
	if g.chance(2) {
		x, y, z := g.name("c"), g.name("c"), g.name("c")
		fmt.Fprintf(b, "const (\n\t%s = iota\n\t%s\n\t%s\n)\n\n", x, y, z)
		add("int", x, y, z)
		g.cgroups = append(g.cgroups, cgroup{"int", []string{x, y, z}})
	}
	if g.chance(2) {
		x, y, z := g.name("c"), g.name("c"), g.name("c")
		fmt.Fprintf(b, "const (\n\t_ = iota\n\t%s = 1 << (3 * iota)\n\t%s\n\t%s\n)\n\n", x, y, z)
		add("int", x, y, z)
		g.cgroups = append(g.cgroups, cgroup{"int", []string{x, y, z}})
	}
	for _, t := range g.named {
		switch {
		case t.base.isInteger() && g.chance(2):
			x, y, z := g.name("c"), g.name("c"), g.name("c")
			fmt.Fprintf(b, "const (\n\t%s %s = iota\n\t%s\n\t%s\n)\n\n", x, t.name, y, z)
			add(t.name, x, y, z)
			g.cgroups = append(g.cgroups, cgroup{t.name, []string{x, y, z}})
		case t.base.kind == kString && g.chance(2):
			x := g.name("c")
			fmt.Fprintf(b, "const %s %s = \"named\"\n\n", x, t.name)
			add(t.name, x)
		}
	}
	if g.chance(2) {
		x := g.name("c")
		fmt.Fprintf(b, "const %s = 3.25\n\n", x)
		add("float64", x)
	}
	if g.chance(2) {
		x := g.name("c")
		fmt.Fprintf(b, "const %s = \"const\" + \"ant\"\n\n", x)
		add("string", x)
	}
	if g.chance(2) {
		x := g.name("c")
		fmt.Fprintf(b, "const %s = 1 < 2\n\n", x)
		add("bool", x)
	}
	if g.chance(2) {
		g.myb = &mybTypes{g.name("MyByte"), g.name("MyBytes"), g.name("MyRunes"), g.name("MyMap")}
		fmt.Fprintf(b, "type %s byte\n\ntype %s []%s\n\ntype %s []rune\n\ntype %s map[string]%s\n\n",
			g.myb.elem, g.myb.bytes, g.myb.elem, g.myb.runes, g.myb.mp, g.myb.elem)
	}
}

// constLeaf returns a constant of the type t, if there is one.
func (g *gen) constLeaf(t *typ) (ex, bool) {
	cs := g.consts[t.String()]
	if len(cs) == 0 || g.chance(3) {
		return ex{}, false
	}
	return ex{s: cs[g.intn(len(cs))], c: true}, true
}

var runeLits = []string{`'a'`, `'Z'`, `'0'`, `'\n'`, `'\t'`, `'\\'`, `'\''`, `'\x41'`, `'é'`, `'\U0001F600'`, `'世'`, `'\000'`}

// shiftCount returns a shift count: a small constant or a masked variable expression of an
// integer type (a negative signed count would panic, and a count at least the width of the
// operand is flagged by vet when it is a constant).
func (g *gen) shiftCount(d int) string {
	switch g.intn(3) {
	case 0:
		return fmt.Sprint(g.intn(8))
	case 1:
		return "(uint(" + g.nonconst(tInt, d-1).s + ") & 7)"
	}
	return "(" + g.nonconst(tInt, d-1).s + " & 7)"
}

// shiftExpr returns an expression of the integer type t with a shift. The shifted value is
// never constant (no constant overflow), or it is the untyped 1 that gets its type from the
// conversion: T(1 << n).
func (g *gen) shiftExpr(t *typ, d int) ex {
	if g.chance(4) {
		cnt := g.shiftCount(d)
		return ex{s: t.String() + "(1 << " + cnt + ")", c: cnt[0] != '('} // a literal count: a constant
	}
	x := g.nonconst(t, d-1)
	return ex{s: "(" + x.s + " " + [2]string{"<<", ">>"}[g.intn(2)] + " " + g.shiftCount(d) + ")"}
}

// sliceExpr3 returns a three-index slice of the type t (a slice), if a slice or an array
// variable of its element type exists.
func (g *gen) sliceExpr3(t *typ, d int) (ex, bool) {
	var c []string
	for _, v := range g.allVars() {
		if (v.t.kind == kSlice || v.t.kind == kArray) && v.t.elem.String() == t.elem.String() {
			c = append(c, v.name)
		}
	}
	if len(c) == 0 {
		return ex{}, false
	}
	name := c[g.intn(len(c))]
	var hi, mx string
	if g.chance(2) {
		hi, mx = "0", "1" // constants in order, within every array (a constant index out of order or out of range is an error)
	} else {
		hi, mx = g.nonconst(tInt, d-1).s, g.nonconst(tInt, d-1).s
	}
	lo := ""
	if g.chance(2) {
		lo = "0"
	}
	return ex{s: name + "[" + lo + ":" + hi + ":" + mx + "]"}, true
}

// extraStmt writes statement kind k of the extras.
func (g *gen) extraStmt(d, k int) {
	switch k {
	case 0, 1:
		g.fwdStmt(d)
	case 2:
		g.constStmt(d)
	case 3:
		g.shiftStmt(d)
	case 4:
		g.runeStmt(d)
	case 5:
		g.namedStmt(d)
	case 6:
		g.methodExprStmt(d)
	case 7:
		g.anonStructStmt(d)
	case 8:
		g.gotoStmt(d)
	}
}

// fwdStmt writes a call f(g()).
func (g *gen) fwdStmt(d int) {
	if len(g.fwds) == 0 {
		return
	}
	p := g.fwds[g.intn(len(g.fwds))]
	call := func() string { return p.src.name + "(" + g.callArgs(p.src, d) + ")" }
	switch g.intn(7) {
	case 0:
		g.line("%s(%s)", p.sink.name, call())
	case 1:
		if len(p.sink.results) == 1 {
			g.decl(p.sink.results[0], ex{s: p.sink.name + "(" + call() + ")"})
			return
		}
		g.line("%s(%s)", p.sink.name, call())
	case 2:
		g.line("defer %s(%s)", p.sink.name, call())
	case 3:
		if g.o.Goroutines {
			g.line("go %s(%s)", p.sink.name, call())
			return
		}
		g.line("%s(%s)", p.sink.name, call())
	case 4:
		g.needFmt = true
		g.line("fmt.Println(%s)", call())
	case 5:
		ft := &typ{kind: kFunc, params: p.sink.params}
		if p.sink.variadic {
			g.line("%s(%s)", p.sink.name, call())
			return
		}
		fv := g.decl(ft, g.funcLit(ft, d))
		g.line("%s(%s)", fv.name, call())
	case 6:
		for _, fm := range g.fwdMeths {
			if fm.src != p.src {
				continue
			}
			for _, v := range g.allVars() {
				if v.t == fm.recv || (v.t.kind == kPtr && v.t.elem == fm.recv) {
					g.line("%s.%s(%s)", v.name, fm.m.name, call())
					return
				}
			}
		}
		g.line("%s(%s)", p.sink.name, call())
	}
}

// constStmt uses the constants: a switch over the constants of a group, an array whose length
// is a constant, and a typed variable initialized from an untyped constant.
func (g *gen) constStmt(d int) {
	if len(g.cgroups) == 0 {
		return
	}
	grp := g.cgroups[g.intn(len(g.cgroups))]
	g.push()
	defer g.pop()
	var tag string
	if grp.key == "int" {
		tag = g.nonconst(tInt, 2).s
	} else {
		for _, t := range g.named {
			if t.name == grp.key {
				tag = g.nonconst(t, 2).s
			}
		}
	}
	g.line("switch %s {", tag)
	n := 1 + g.intn(len(grp.names))
	for i := 0; i < n; i++ {
		g.line("case %s:", grp.names[i])
		g.scoped(d, 1, nil)
	}
	if g.chance(2) {
		g.line("default:")
		g.scoped(d, 1, nil)
	}
	g.line("}")
	if grp.key == "int" {
		a := g.name("v")
		g.line("var %s [%s + 1]int", a, grp.names[g.intn(len(grp.names))])
		g.line("_ = %s", a)
		f := g.name("v")
		g.line("var %s float64 = %s", f, grp.names[g.intn(len(grp.names))])
		g.line("_ = %s", f)
	}
}

// shiftStmt writes a shift assignment, x <<= n or x >>= n, on an integer variable.
func (g *gen) shiftStmt(d int) {
	var c []lvalue
	for _, l := range g.lvalues(d) {
		if l.t.isInteger() {
			c = append(c, l)
		}
	}
	if len(c) == 0 {
		return
	}
	l := c[g.intn(len(c))]
	g.line("%s %s %s", l.s, [2]string{"<<=", ">>="}[g.intn(2)], g.shiftCount(d))
}

// runeStmt writes rune literals, conversions between runes, strings and []rune, and a switch on
// rune constants.
func (g *gen) runeStmt(d int) {
	g.line("{")
	g.ind++
	r, r2, s, rs := g.name("r"), g.name("r"), g.name("s"), g.name("rs")
	g.line("%s := %s", r, runeLits[g.intn(len(runeLits))])
	g.line("var %s rune = %s + %s", r2, r, g.nonconst(tInt32, 1).s)
	g.line("%s := string(%s)", s, r2)
	g.line("%s := []rune(%s + %s)", rs, s, g.nonconst(tString, 1).s)
	g.line("_, _, _, _ = %s, %s, %s, %s", r, r2, s, rs)
	g.line("%s = append(%s, %s)", rs, rs, runeLits[g.intn(len(runeLits))])
	g.line("_ = string(%s)", rs)
	g.line("_ = []byte(%s)", s)
	if g.chance(2) {
		g.line("for _, c := range %q {", "héllo 世界")
		g.line("\tif c == %s {", runeLits[g.intn(len(runeLits))])
		g.line("\t\tbreak")
		g.line("\t}")
		g.line("}")
	}
	if g.chance(2) {
		a := g.intn(len(runeLits))
		b := (a + 1 + g.intn(len(runeLits)-1)) % len(runeLits)
		g.line("switch %s {", r)
		g.line("case %s, %s:", runeLits[a], runeLits[b])
		g.line("\t_ = %s", r)
		g.line("default:")
		g.line("}")
	}
	if g.chance(2) {
		bb := g.name("b")
		g.line("var %s byte = 'a' + byte(%s&15)", bb, r)
		g.line("_ = %s", bb)
	}
	g.ind--
	g.line("}")
}

// namedStmt uses the declared named slice, map and element types.
func (g *gen) namedStmt(d int) {
	m := g.myb
	if m == nil {
		return
	}
	g.line("{")
	g.ind++
	mb, mr, mm, s := g.name("v"), g.name("v"), g.name("v"), g.name("v")
	if g.o.Gaps {
		g.line("%s := %s(%s)", mb, m.bytes, g.nonconst(tString, 1).s)
	} else {
		g.line("%s := %s{1, 2, %s(len(%s))}", mb, m.bytes, m.elem, g.nonconst(tString, 1).s)
	}
	g.line("%s := %s(%s)", mr, m.runes, g.nonconst(tString, 1).s)
	g.line("%s := %s{}", mm, m.mp)
	g.line("_, _, _ = %s, %s, %s", mb, mr, mm)
	if g.o.Gaps {
		g.line("%s := string(%s) + string(%s)", s, mb, mr)
	} else {
		g.line("%s := string(%s)", s, mr)
	}
	g.line("_ = %s", s)
	if g.chance(2) {
		g.line("%s = append(%s, %s(%s&127), 1, 2)", mb, mb, m.elem, g.nonconst(tInt, 1).s)
	}
	if g.o.Gaps && g.chance(2) {
		g.line("%s = append(%s, []%s(%s)...)", mb, mb, m.elem, s)
	}
	if g.o.Gaps && g.chance(2) {
		g.line("copy(%s, %s(%s))", mb, m.bytes, s)
	}
	if !g.o.Gaps && g.chance(2) {
		g.line("copy(%s, %s{3})", mb, m.bytes)
	}
	if g.chance(2) {
		g.line("%s[%q] = %s(len(%s))", mm, "k", m.elem, mb)
		g.line("if x, ok := %s[%q]; ok {", mm, "k")
		g.line("\t_ = x + 1")
		g.line("}")
		g.line("for k, x := range %s {", mm)
		g.line("\t_, _ = k, x")
		g.line("}")
		g.line("delete(%s, %q)", mm, "k")
	}
	if g.chance(2) {
		g.line("for i, x := range %s {", mb)
		g.line("\t_, _ = i, x")
		g.line("}")
		g.line("_ = %s[:len(%s)/2]", mb, mb)
	}
	if g.chance(2) {
		g.line("_ = len(%s) + cap(%s)", mr, mr)
		g.line("_ = %s(string(%s[:0]))", m.runes, mr)
	}
	g.ind--
	g.line("}")
}

// methodExprStmt calls a method through a method expression, T.m or (*T).m.
func (g *gen) methodExprStmt(d int) {
	type cand struct {
		t *typ
		m member
	}
	var c []cand
	for _, t := range append(append([]*typ{}, g.structs...), g.named...) {
		all := t.allMethods()
		for _, m := range all {
			if t.kind == kNamed && m.m.ptr {
				continue
			}
			same := 0 // a method reached by two paths (a diamond of embedded structs) is an ambiguous selector
			for _, o := range all {
				if o.m.name == m.m.name {
					same++
				}
			}
			if same == 1 {
				c = append(c, cand{t, m})
			}
		}
	}
	for _, it := range g.ifaces {
		for _, m := range it.methods {
			c = append(c, cand{it, member{m: m}})
		}
	}
	if len(c) == 0 {
		return
	}
	x := c[g.intn(len(c))]
	t, m := x.t, x.m.m
	recvT := t
	expr := t.String() + "." + m.name
	if m.ptr {
		recvT = &typ{kind: kPtr, elem: t}
		expr = "(*" + t.String() + ")." + m.name
	} else if t.kind == kStruct && g.chance(3) {
		recvT = &typ{kind: kPtr, elem: t}
		expr = "(*" + t.String() + ")." + m.name
	}
	f := g.name("v")
	g.line("%s := %s", f, expr)
	g.line("_ = %s", f)
	args := g.expr(recvT, 2).s
	if len(m.params) > 0 {
		args += ", " + g.args(m.params, 2)
	}
	if len(m.results) == 1 && g.chance(2) {
		g.decl(m.results[0], ex{s: f + "(" + args + ")"})
		return
	}
	g.line("%s(%s)", f, args)
}

// anonStructStmt uses anonymous struct types: literals, assignment between identical types,
// pointers, slices and maps of them, a function parameter, comparison, conversion to a named
// struct type, and the empty struct.
func (g *gen) anonStructStmt(d int) {
	ts := g.scalarTypes()
	n := 1 + g.intn(3)
	var fields, vals []string
	var ftypes []*typ
	for i := 0; i < n; i++ {
		t := ts[g.intn(len(ts))]
		ftypes = append(ftypes, t)
		fields = append(fields, fmt.Sprintf("f%d %s", i, t))
		vals = append(vals, g.expr(t, 2).s)
	}
	st := "struct{ " + strings.Join(fields, "; ") + " }"
	a, b := g.name("v"), g.name("v")
	g.line("{")
	g.ind++
	g.line("%s := %s{%s}", a, st, strings.Join(vals, ", "))
	g.line("%s := %s{f0: %s}", b, st, vals[0])
	g.line("%s = %s", a, b)
	g.line("pa := &%s", a)
	g.line("_ = pa.f0")
	g.line("sl := []%s{{%s}, {%s}}", st, strings.Join(vals, ", "), strings.Join(vals, ", "))
	g.line("for _, e := range sl {")
	g.line("\t_ = e.f0")
	g.line("}")
	g.line("mp := map[string]%s{%q: {%s}}", st, "a", strings.Join(vals, ", "))
	g.line("_ = mp[%q].f0", "a")
	g.line("fn := func(p %s) int { _ = p.f0; return %d }", st, g.intn(10))
	g.line("_ = fn(%s)", a)
	if ftypes[0].comparable() {
		g.line("_ = %s == %s", a, b)
	}
	if g.chance(2) {
		g.line("type local %s", st)
		g.line("_ = local(%s)", a)
	}
	if g.chance(2) {
		g.line("var e struct{}")
		g.line("_ = e")
	}
	g.ind--
	g.line("}")
}

// gotoStmt writes a backward goto (a loop) and a forward one over a block.
func (g *gen) gotoStmt(d int) {
	g.line("{")
	g.ind++
	if g.chance(2) {
		l, n := g.name("G"), g.name("n")
		g.line("%s := 0", n)
		g.line("%s:", l)
		g.line("%s++", n)
		g.line("if %s < %d {", n, 1+g.intn(3))
		g.line("\tgoto %s", l)
		g.line("}")
		g.line("_ = %s", n)
	} else {
		l, n := g.name("G"), g.name("n")
		g.line("%s := %s", n, g.nonconst(tInt, 1).s)
		g.line("if %s {", g.cond(2))
		g.line("\tgoto %s", l)
		g.line("}")
		g.line("{")
		g.scoped(d, 1+g.intn(2), nil)
		g.line("}")
		g.line("%s:", l)
		g.line("_ = %s", n)
	}
	g.ind--
	g.line("}")
}
