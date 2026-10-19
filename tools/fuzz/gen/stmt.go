package gen

import (
	"fmt"
	"strings"
)

// ret writes the return statement that ends a function body.
func (g *gen) ret() {
	if len(g.res) == 0 {
		return
	}
	var parts []string
	for _, r := range g.res {
		parts = append(parts, g.expr(r, 2).s)
	}
	g.line("return %s", strings.Join(parts, ", "))
}

// block writes n statements.
func (g *gen) block(d, n int) {
	for i := 0; i < n; i++ {
		g.stmt(d)
	}
}

// scoped writes { block } as the body of a statement whose first line is already written.
func (g *gen) scoped(d, n int, before func()) {
	g.ind++
	g.push()
	if before != nil {
		before()
	}
	g.block(d-1, n)
	g.pop()
	g.ind--
}

func isDefault(t *typ) bool {
	switch t.kind {
	case kInt, kString, kBool, kFloat:
		return true
	}
	return false
}

// decl writes the declaration of a new variable of type t with the value e, and uses it.
func (g *gen) decl(t *typ, e ex) *variable {
	name := g.name("v")
	if isDefault(t) {
		g.line("%s := %s", name, e.s)
	} else {
		g.line("var %s %s = %s", name, t, e.s)
	}
	v := g.declare(name, t)
	g.line("_ = %s", name)
	return v
}

type lvalue struct {
	s string
	t *typ
}

// lvalues returns what an assignment can write to.
func (g *gen) lvalues(d int) []lvalue {
	var out []lvalue
	for _, v := range g.allVars() {
		out = append(out, lvalue{v.name, v.t})
		vt := v.t
		if vt.kind == kPtr {
			out = append(out, lvalue{"*" + v.name, vt.elem})
			vt = vt.elem
		}
		if vt.kind == kStruct {
			for _, m := range vt.allFields() {
				out = append(out, lvalue{v.name + "." + m.path + m.f.name, m.f.t})
			}
		}
		switch v.t.kind {
		case kSlice:
			out = append(out, lvalue{v.name + "[" + g.idx(d-1) + "]", v.t.elem})
		case kMap:
			out = append(out, lvalue{v.name + "[" + g.expr(v.t.key, d-1).s + "]", v.t.elem})
		case kArray:
			out = append(out, lvalue{fmt.Sprintf("%s[%d]", v.name, g.intn(v.t.n)), v.t.elem})
		}
	}
	return out
}

func (g *gen) cond(d int) string { return g.nonconst(tBool, d).s }

func (g *gen) stmt(d int) {
	g.budget--
	k := g.intn(24)
	if g.o.Extras && g.chance(3) {
		k = 24 + g.intn(9)
	}
	if d <= 0 || g.budget <= 0 {
		k %= 5
	}
	switch k {
	case 0:
		t := g.valueType(2, nil)
		g.decl(t, g.expr(t, 3))
	case 1:
		t := g.valueType(2, nil)
		name := g.name("v")
		g.line("var %s %s", name, t)
		g.declare(name, t)
		g.line("_ = %s", name)
	case 2:
		ls := g.lvalues(d)
		if len(ls) > 0 {
			l := ls[g.intn(len(ls))]
			g.line("%s = %s", l.s, g.expr(l.t, 3).s)
		}
	case 3:
		g.opAssign(d)
	case 4:
		g.callStmt(d)
	case 5, 6:
		g.ifStmt(d)
	case 7:
		g.forStmt(d)
	case 8:
		g.rangeStmt(d)
	case 9:
		g.switchStmt(d)
	case 10:
		g.typeSwitch(d)
	case 11:
		g.closureStmt(d)
	case 12:
		g.deferStmt(d)
	case 13:
		if g.o.Goroutines {
			g.goStmt(d)
		}
	case 14:
		if g.o.Goroutines {
			g.chanStmt(d)
		}
	case 15:
		if g.o.Goroutines {
			g.selectStmt(d)
		}
	case 16:
		g.multiAssign(d)
	case 17:
		g.labeled(d)
	case 18:
		g.line("{")
		g.scoped(d, 1+g.intn(2), nil)
		g.line("}")
	case 19:
		g.collectionStmt(d)
	case 20:
		if len(g.res) > 0 {
			g.line("if %s {", g.cond(2))
			g.ind++
			g.ret()
			g.ind--
			g.line("}")
		}
	case 21:
		if g.breaks > 0 {
			kw := "break"
			if g.loops > 0 && g.chance(2) {
				kw = "continue"
			}
			g.line("if %s {", g.cond(2))
			g.line("\t%s", kw)
			g.line("}")
		}
	case 22:
		g.recoverStmt(d)
	case 23:
		g.commaOk(d)
	default:
		g.extraStmt(d, k-24)
	}
}

func (g *gen) opAssign(d int) {
	var c []lvalue
	for _, l := range g.lvalues(d) {
		if l.t.isNumeric() || l.t.isString() {
			c = append(c, l)
		}
	}
	if len(c) == 0 {
		return
	}
	l := c[g.intn(len(c))]
	switch {
	case l.t.isString():
		g.line("%s += %s", l.s, g.expr(l.t, 2).s)
	case g.chance(4):
		g.line("%s%s", l.s, [2]string{"++", "--"}[g.intn(2)])
	default:
		ops := []string{"+=", "-=", "*="}
		if l.t.isInteger() {
			ops = append(ops, "&=", "|=", "^=", "/=", "%=")
		} else {
			ops = append(ops, "/=")
		}
		op := ops[g.intn(len(ops))]
		e := g.expr(l.t, 2)
		if op == "/=" || op == "%=" {
			if l.t.isInteger() {
				e = ex{s: "(" + e.s + " | 1)", c: e.c}
			} else {
				e = ex{s: "(" + e.s + " + 1.5)", c: e.c}
			}
		}
		g.line("%s %s %s", l.s, op, e.s)
	}
}

// callStmt writes a call whose results (if any) are dropped.
func (g *gen) callStmt(d int) {
	var c []func()
	for _, f := range g.funcs {
		f := f
		c = append(c, func() { g.line("%s(%s)", f.name, g.callArgs(f, d)) })
	}
	for _, v := range g.allVars() {
		v := v
		vt := v.t
		if vt.kind == kFunc {
			c = append(c, func() { g.line("%s(%s)", v.name, g.args(vt.params, d)) })
		}
		recv := vt
		if recv.kind == kPtr {
			recv = recv.elem
		}
		if recv.kind == kStruct || recv.kind == kNamed {
			for _, m := range recv.allMethods() {
				m := m
				if recv.kind == kNamed && m.m.ptr {
					continue
				}
				c = append(c, func() { g.line("%s.%s%s(%s)", v.name, m.path, m.m.name, g.args(m.m.params, d)) })
			}
		}
		if vt.kind == kIface {
			for _, m := range vt.methods {
				m := m
				c = append(c, func() { g.line("%s.%s(%s)", v.name, m.name, g.args(m.params, d)) })
			}
		}
	}
	c = append(c, func() {
		g.needFmt = true
		t := g.valueType(1, nil)
		g.line("fmt.Println(%s)", g.expr(t, 2).s)
	})
	c[g.intn(len(c))]()
}

func (g *gen) ifStmt(d int) {
	g.push()
	defer g.pop()
	var init string
	var iv *variable
	if g.chance(3) {
		t := []*typ{tInt, tString, tBool, tFloat}[g.intn(4)]
		name := g.name("v")
		init = name + " := " + g.expr(t, 2).s + "; "
		iv = g.declare(name, t)
	}
	g.line("if %s%s {", init, g.cond(3))
	g.scoped(d, 1+g.intn(3), func() {
		if iv != nil {
			g.line("_ = %s", iv.name)
		}
	})
	for n := 0; n < 3 && g.chance(4); n++ { // bounded: a source that is exhausted says "yes" to everything
		g.line("} else if %s {", g.cond(2))
		g.scoped(d, 1+g.intn(2), nil)
	}
	if g.chance(2) {
		g.line("} else {")
		g.scoped(d, 1+g.intn(2), nil)
	}
	g.line("}")
}

func (g *gen) loopBody(d int, before func()) {
	g.loops++
	g.breaks++
	g.scoped(d, 1+g.intn(3), before)
	g.loops--
	g.breaks--
}

func (g *gen) forStmt(d int) {
	g.push()
	defer g.pop()
	switch g.intn(4) {
	case 0, 1:
		i := g.name("i")
		var limit string
		if vs := g.sliceVars(); len(vs) > 0 && g.chance(2) {
			limit = "len(" + vs[g.intn(len(vs))].name + ")"
		} else {
			limit = fmt.Sprint(1 + g.intn(4))
		}
		g.line("for %s := 0; %s < %s; %s++ {", i, i, limit, i)
		iv := g.declare(i, tInt)
		g.loopBody(d, func() { g.line("_ = %s", iv.name) })
	case 2:
		g.line("for %s {", g.cond(2))
		g.loopBody(d, nil)
	case 3:
		g.line("for {")
		g.loops++
		g.breaks++
		g.scoped(d, 1+g.intn(2), nil)
		g.ind++
		g.line("if %s {", g.cond(2))
		g.line("\tbreak")
		g.line("}")
		g.ind--
		g.loops--
		g.breaks--
	}
	g.line("}")
}

func (g *gen) rangeStmt(d int) {
	g.push()
	defer g.pop()
	var c []func()
	for _, v := range g.allVars() {
		v := v
		switch v.t.kind {
		case kSlice, kArray:
			c = append(c, func() {
				i, x := g.name("i"), g.name("e")
				g.line("for %s, %s := range %s {", i, x, v.name)
				iv, xv := g.declare(i, tInt), g.declare(x, v.t.elem)
				g.loopBody(d, func() { g.line("_ = %s", iv.name); g.line("_ = %s", xv.name) })
				g.line("}")
			}, func() {
				i := g.name("i")
				g.line("for %s := range %s {", i, v.name)
				iv := g.declare(i, tInt)
				g.loopBody(d, func() { g.line("_ = %s", iv.name) })
				g.line("}")
			})
		case kString:
			c = append(c, func() {
				i, r := g.name("i"), g.name("r")
				g.line("for %s, %s := range %s {", i, r, v.name)
				iv, rvv := g.declare(i, tInt), g.declare(r, &typ{kind: kInt32})
				g.loopBody(d, func() { g.line("_ = %s", iv.name); g.line("_ = %s", rvv.name) })
				g.line("}")
			})
		case kMap:
			c = append(c, func() {
				k, x := g.name("k"), g.name("e")
				g.line("for %s, %s := range %s {", k, x, v.name)
				kv, xv := g.declare(k, v.t.key), g.declare(x, v.t.elem)
				g.loopBody(d, func() { g.line("_ = %s", kv.name); g.line("_ = %s", xv.name) })
				g.line("}")
			})
		case kChan:
			c = append(c, func() {
				x := g.name("e")
				g.line("for %s := range %s {", x, v.name)
				xv := g.declare(x, v.t.elem)
				g.loopBody(d, func() { g.line("_ = %s", xv.name) })
				g.line("}")
			})
		}
	}
	c = append(c, func() {
		i := g.name("i")
		n := fmt.Sprint(1 + g.intn(4))
		if g.chance(2) {
			n = g.nonconst(tInt, 1).s
		}
		g.line("for %s := range %s {", i, n)
		iv := g.declare(i, tInt)
		g.loopBody(d, func() { g.line("_ = %s", iv.name) })
		g.line("}")
	}, func() {
		g.line("for range %d {", 1+g.intn(3))
		g.loopBody(d, nil)
		g.line("}")
	})
	if g.o.RangeFunc {
		c = append(c, func() {
			seq, x := g.name("seq"), g.name("e")
			g.line("%s := func(yield func(int) bool) {", seq)
			g.line("\tif !yield(%d) {", g.intn(10))
			g.line("\t\treturn")
			g.line("\t}")
			g.line("}")
			g.line("_ = %s", seq)
			g.line("for %s := range %s {", x, seq)
			xv := g.declare(x, tInt)
			g.loopBody(d, func() { g.line("_ = %s", xv.name) })
			g.line("}")
		})
	}
	c[g.intn(len(c))]()
}

func (g *gen) switchStmt(d int) {
	g.push()
	defer g.pop()
	tagless := g.chance(2)
	var tag *typ
	if tagless {
		g.line("switch {")
	} else {
		tag = []*typ{tInt, tString}[g.intn(2)]
		g.line("switch %s {", g.expr(tag, 2).s)
	}
	n := 1 + g.intn(3)
	used := 0
	def := g.chance(2)
	total := n
	if def {
		total++
	}
	defAt := g.intn(total)
	ci := 0
	for i := 0; i < total; i++ {
		last := i == total-1
		if def && i == defAt {
			g.line("default:")
		} else if tagless {
			g.line("case %s:", g.cond(2))
		} else {
			var vals []string
			for k := 1 + g.intn(2); k > 0; k-- {
				ci++
				if tag.kind == kString {
					vals = append(vals, fmt.Sprintf("%q", fmt.Sprint("c", ci)))
				} else {
					vals = append(vals, fmt.Sprint(ci))
				}
			}
			g.line("case %s:", strings.Join(vals, ", "))
		}
		used++
		g.breaks++
		g.scoped(d, 1+g.intn(2), nil)
		g.breaks--
		if !last && g.chance(4) {
			g.ind++
			g.line("fallthrough")
			g.ind--
		}
	}
	g.line("}")
}

func (g *gen) typeSwitch(d int) {
	g.push()
	defer g.pop()
	var c []*variable
	for _, v := range g.allVars() {
		if v.t.kind == kAny || (v.t.kind == kIface && len(v.t.impls) > 0) {
			c = append(c, v)
		}
	}
	if len(c) == 0 {
		return
	}
	v := c[g.intn(len(c))]
	var cases []*typ
	if v.t.kind == kIface {
		for _, s := range v.t.impls {
			if len(cases) < 3 && (len(cases) == 0 || g.chance(2)) {
				cases = append(cases, &typ{kind: kPtr, elem: s})
			}
		}
	} else {
		pool := []*typ{tInt, tString, tBool, tFloat, {kind: kSlice, elem: tInt}, tInt64}
		if len(g.structs) > 0 {
			pool = append(pool, &typ{kind: kPtr, elem: g.structs[g.intn(len(g.structs))]})
		}
		seen := map[string]bool{}
		for k := 1 + g.intn(3); k > 0; k-- {
			t := pool[g.intn(len(pool))]
			if !seen[t.String()] {
				seen[t.String()] = true
				cases = append(cases, t)
			}
		}
	}
	x := g.name("x")
	g.line("switch %s := %s.(type) {", x, v.name)
	g.breaks++
	for _, t := range cases {
		g.line("case %s:", t)
		g.scoped(d, 1+g.intn(2), func() {
			g.declare(x, t)
			g.line("_ = %s", x)
		})
	}
	if g.chance(3) {
		g.line("case nil:")
		g.scoped(d, 1, func() { g.line("_ = %s", x) })
	}
	if g.chance(2) {
		g.line("default:")
		g.scoped(d, 1, func() { g.line("_ = %s", x) })
	}
	g.breaks--
	g.line("}")
}

func (g *gen) closureStmt(d int) {
	ft := &typ{kind: kFunc}
	for k := g.intn(3); k > 0; k-- {
		ft.params = append(ft.params, g.valueType(1, nil))
	}
	if g.chance(2) {
		ft.results = append(ft.results, g.valueType(1, nil))
	}
	e := g.funcLit(ft, d)
	v := g.decl(ft, e)
	switch {
	case len(ft.results) > 0 && g.chance(2):
		g.decl(ft.results[0], ex{s: v.name + "(" + g.args(ft.params, 2) + ")"})
	case g.chance(2):
		g.line("%s(%s)", v.name, g.args(ft.params, 2))
	}
}

func (g *gen) deferStmt(d int) {
	switch g.intn(3) {
	case 0:
		lit := g.funcLit(&typ{kind: kFunc}, d)
		g.line("defer %s()", lit.s)
	case 1:
		if len(g.funcs) > 0 {
			f := g.funcs[g.intn(len(g.funcs))]
			g.line("defer %s(%s)", f.name, g.callArgs(f, d))
			return
		}
		fallthrough
	default:
		g.recoverStmt(d)
	}
}

func (g *gen) recoverStmt(d int) {
	r := g.name("r")
	g.line("defer func() {")
	g.line("\tif %s := recover(); %s != nil {", r, r)
	g.line("\t\t_ = %s", r)
	g.line("\t}")
	g.line("}()")
	if g.chance(3) {
		g.line("panic(%s)", g.expr([]*typ{tString, tInt}[g.intn(2)], 1).s)
	}
}

func (g *gen) goStmt(d int) {
	if g.chance(3) && len(g.funcs) > 0 {
		f := g.funcs[g.intn(len(g.funcs))]
		g.line("go %s(%s)", f.name, g.callArgs(f, d))
		return
	}
	ft := &typ{kind: kFunc}
	for k := g.intn(2); k > 0; k-- {
		ft.params = append(ft.params, g.valueType(1, nil))
	}
	lit := g.funcLit(ft, d)
	g.line("go %s(%s)", lit.s, g.args(ft.params, 2))
}

func (g *gen) chanVars() []*variable {
	var out []*variable
	for _, v := range g.allVars() {
		if v.t.kind == kChan {
			out = append(out, v)
		}
	}
	return out
}

// anyChan returns a channel variable, declaring a buffered one if there is none.
func (g *gen) anyChan() *variable {
	if cs := g.chanVars(); len(cs) > 0 && !g.chance(3) {
		return cs[g.intn(len(cs))]
	}
	t := &typ{kind: kChan, elem: g.valueType(1, nil)}
	name := g.name("ch")
	g.line("%s := make(%s, %d)", name, t, 1+g.intn(3))
	v := g.declare(name, t)
	g.line("_ = %s", name)
	return v
}

func (g *gen) chanStmt(d int) {
	ch := g.anyChan()
	switch g.intn(5) {
	case 0, 1:
		g.line("%s <- %s", ch.name, g.expr(ch.t.elem, 2).s)
	case 2:
		g.line("<-%s", ch.name)
	case 3:
		x, ok := g.name("v"), g.name("ok")
		g.line("%s, %s := <-%s", x, ok, ch.name)
		g.declare(x, ch.t.elem)
		g.declare(ok, tBool)
		g.line("_, _ = %s, %s", x, ok)
	case 4:
		g.line("close(%s)", ch.name)
	}
}

func (g *gen) selectStmt(d int) {
	g.push()
	defer g.pop()
	ch := g.anyChan()
	g.line("select {")
	g.breaks++
	n := 1 + g.intn(3)
	hasDef := false
	for i := 0; i < n; i++ {
		switch g.intn(4) {
		case 0:
			g.line("case <-%s:", ch.name)
			g.scoped(d, 1, nil)
		case 1:
			x := g.name("v")
			g.line("case %s := <-%s:", x, ch.name)
			g.scoped(d, 1, func() {
				g.declare(x, ch.t.elem)
				g.line("_ = %s", x)
			})
		case 2:
			g.line("case %s <- %s:", ch.name, g.expr(ch.t.elem, 2).s)
			g.scoped(d, 1, nil)
		case 3:
			if !hasDef {
				hasDef = true
				g.line("default:")
				g.scoped(d, 1, nil)
			}
		}
	}
	g.breaks--
	g.line("}")
}

func (g *gen) multiAssign(d int) {
	var c []*fn
	for _, f := range g.funcs {
		if len(f.results) >= 2 {
			c = append(c, f)
		}
	}
	if len(c) == 0 {
		g.callStmt(d)
		return
	}
	f := c[g.intn(len(c))]
	var names []string
	for _, r := range f.results {
		n := g.name("v")
		names = append(names, n)
		defer g.declare(n, r)
	}
	g.line("%s := %s(%s)", strings.Join(names, ", "), f.name, g.callArgs(f, d))
	g.line("_, _ = %s, %s", names[0], names[1])
	for _, n := range names[2:] {
		g.line("_ = %s", n)
	}
}

func (g *gen) commaOk(d int) {
	var c []func()
	for _, v := range g.allVars() {
		v := v
		if v.t.kind == kMap {
			c = append(c, func() {
				x, ok := g.name("v"), g.name("ok")
				g.line("%s, %s := %s[%s]", x, ok, v.name, g.expr(v.t.key, 2).s)
				g.declare(x, v.t.elem)
				g.declare(ok, tBool)
				g.line("_, _ = %s, %s", x, ok)
			})
		}
		if v.t.kind == kAny {
			c = append(c, func() {
				x, ok := g.name("v"), g.name("ok")
				t := []*typ{tInt, tString, tFloat}[g.intn(3)]
				g.line("%s, %s := %s.(%s)", x, ok, v.name, t)
				g.declare(x, t)
				g.declare(ok, tBool)
				g.line("_, _ = %s, %s", x, ok)
			})
		}
	}
	if len(c) > 0 {
		c[g.intn(len(c))]()
	}
}

// collectionStmt writes append, delete, clear or copy.
func (g *gen) collectionStmt(d int) {
	var c []func()
	for _, v := range g.allVars() {
		v := v
		switch v.t.kind {
		case kSlice:
			c = append(c, func() {
				g.line("%s = append(%s, %s)", v.name, v.name, g.elems(v.t.elem, 1+g.intn(3), d))
			}, func() {
				g.line("copy(%s, %s)", v.name, g.expr(v.t, 2).s)
			}, func() {
				g.line("clear(%s)", v.name)
			})
		case kMap:
			c = append(c, func() {
				g.line("delete(%s, %s)", v.name, g.expr(v.t.key, 2).s)
			}, func() {
				g.line("clear(%s)", v.name)
			})
		}
	}
	if len(c) > 0 {
		c[g.intn(len(c))]()
	}
}

func (g *gen) labeled(d int) {
	g.push()
	defer g.pop()
	l := g.name("L")
	i, j := g.name("i"), g.name("j")
	g.line("%s:", l)
	g.line("for %s := 0; %s < %d; %s++ {", i, i, 1+g.intn(3), i)
	iv := g.declare(i, tInt)
	g.loops++
	g.breaks++
	g.scoped(d, 1, func() {
		g.line("_ = %s", iv.name)
		g.line("for %s := 0; %s < %d; %s++ {", j, j, 1+g.intn(3), j)
		g.ind++
		g.push()
		jv := g.declare(j, tInt)
		g.line("_ = %s", jv.name)
		g.line("if %s {", g.cond(2))
		g.line("\t%s %s", [2]string{"break", "continue"}[g.intn(2)], l)
		g.line("}")
		g.block(d-1, 1)
		g.pop()
		g.ind--
		g.line("}")
	})
	g.loops--
	g.breaks--
	g.line("}")
}

// writeMethods writes the methods of the structs and the named types.
func (g *gen) writeMethods() {
	for _, t := range append(append([]*typ{}, g.structs...), g.named...) {
		for _, m := range t.methods {
			recv := t.String()
			rt := t
			if m.ptr {
				recv = "*" + recv
				rt = &typ{kind: kPtr, elem: t}
			}
			g.push()
			g.res = m.results
			var ps []string
			for _, p := range m.params {
				n := g.name("p")
				ps = append(ps, n+" "+p.String())
				g.declare(n, p)
			}
			g.declare("rcv", rt)
			g.line("func (rcv %s) %s(%s)%s {", recv, m.name, strings.Join(ps, ", "), resultList(m.results))
			g.ind++
			g.block(3, 1+g.intn(4))
			g.ret()
			g.ind--
			g.line("}")
			g.line("")
			g.pop()
		}
	}
}

// writeFuncs writes the functions.
func (g *gen) writeFuncs() {
	for _, f := range g.funcs {
		g.push()
		g.res = f.results
		var ps []string
		for i, p := range f.params {
			n := g.name("p")
			if f.variadic && i == len(f.params)-1 {
				ps = append(ps, n+" ..."+p.elem.String())
			} else {
				ps = append(ps, n+" "+p.String())
			}
			g.declare(n, p)
		}
		// Sometimes named results, and a deferred closure that changes them.
		named := len(f.results) > 0 && g.chance(4)
		var rn []string
		sig := resultList(f.results)
		if named {
			var parts []string
			for _, r := range f.results {
				n := g.name("r")
				rn = append(rn, n)
				parts = append(parts, n+" "+r.String())
			}
			sig = " (" + strings.Join(parts, ", ") + ")"
		}
		g.line("func %s(%s)%s {", f.name, strings.Join(ps, ", "), sig)
		g.ind++
		if named {
			for i, n := range rn {
				g.declare(n, f.results[i])
			}
			g.line("defer func() {")
			g.line("\t%s = %s", rn[0], g.expr(f.results[0], 2).s)
			g.line("}()")
		}
		g.block(3, 1+g.intn(5))
		if f.fwd != nil {
			g.line("return %s(%s)", f.fwd.name, g.callArgs(f.fwd, 2))
		} else {
			g.ret()
		}
		g.ind--
		g.line("}")
		g.line("")
		g.pop()
	}
}

func (g *gen) writeMain() {
	g.push()
	g.res = nil
	g.line("func main() {")
	g.ind++
	g.block(3, 4+g.intn(8))
	g.line("fmt.Println(%s, %s)", g.expr(tInt, 2).s, g.expr(tString, 2).s)
	g.ind--
	g.line("}")
	g.pop()
}
