package gen

import (
	"fmt"
	"strings"
)

// An ex is an expression: its text and whether it is a constant (untyped or typed). A constant
// expression is checked at compile time (overflow, division by zero, conversions, indexes), so
// the generator keeps a variable in every expression that could fail that check.
type ex struct {
	s string
	c bool
}

// scalarVar returns the global variable of the scalar type t.
func (g *gen) scalarVar(t *typ) ex {
	return ex{s: g.scalarV[t.String()].name}
}

// nonconst returns an expression of the scalar type t that is not a constant.
func (g *gen) nonconst(t *typ, d int) ex {
	e := g.expr(t, d)
	if !e.c {
		return e
	}
	v := g.scalarVar(t)
	switch {
	case t.isBool():
		return ex{s: "(" + v.s + " && " + e.s + ")"}
	case t.isString() || t.isNumeric():
		return ex{s: "(" + v.s + " + " + e.s + ")"}
	}
	panic("gen: nonconst of " + t.String())
}

// idx returns an index: a small constant or an int expression with a variable.
func (g *gen) idx(d int) string {
	if g.chance(2) {
		return fmt.Sprint(g.intn(3))
	}
	return g.nonconst(tInt, d).s
}

// leafLit returns a literal (or the zero value) of the type t.
func (g *gen) leafLit(t *typ) ex {
	if g.o.Extras {
		if e, ok := g.constLeaf(t); ok {
			return e
		}
		if t.kind == kInt32 && g.chance(3) {
			return ex{s: runeLits[g.intn(len(runeLits))], c: true}
		}
	}
	switch t.kind {
	case kBool:
		return ex{s: [2]string{"true", "false"}[g.intn(2)], c: true}
	case kString:
		return ex{s: fmt.Sprintf("%q", [4]string{"", "a", "go", "gocpp"}[g.intn(4)]), c: true}
	case kFloat:
		// typed: an untyped constant would give a variable declared with := the type int
		return ex{s: "float64(" + [4]string{"0", "1.5", "2.25", "10"}[g.intn(4)] + ")", c: true}
	case kInt:
		return ex{s: fmt.Sprint(g.intn(100)), c: true}
	case kInt64, kInt32, kUint8, kUint:
		return ex{s: t.String() + "(" + fmt.Sprint(g.intn(100)) + ")", c: true}
	case kNamed:
		return ex{s: t.name + "(" + g.leafLit(t.base).s + ")", c: true}
	case kSlice, kMap, kArray, kStruct:
		return ex{s: t.String() + "{}"}
	case kPtr:
		if t.elem.kind == kStruct {
			return ex{s: "&" + t.elem.String() + "{}"}
		}
		return ex{s: "new(" + t.elem.String() + ")"}
	case kChan:
		return ex{s: "make(" + t.String() + ")"}
	case kFunc, kIface:
		return ex{s: "(" + t.String() + ")(nil)"}
	case kAny:
		return ex{s: "any(" + g.leafLit(tInt).s + ")"}
	}
	panic("gen: leafLit " + t.String())
}

// leaf returns a variable of type t, or else a literal.
func (g *gen) leaf(t *typ) ex {
	if vs := g.varsOf(t); len(vs) > 0 && g.intn(3) > 0 {
		return ex{s: vs[g.intn(len(vs))].name}
	}
	return g.leafLit(t)
}

// expr returns an expression of type t, of depth about d.
func (g *gen) expr(t *typ, d int) ex {
	if d <= 0 {
		return g.leaf(t)
	}
	switch {
	case t.isNumeric():
		return g.numExpr(t, d)
	case t.isString():
		return g.strExpr(t, d)
	case t.isBool():
		return g.boolExpr(t, d)
	}
	return g.compExpr(t, d)
}

func (g *gen) numExpr(t *typ, d int) ex {
	n := 10
	if g.o.Extras && t.isInteger() {
		n = 13
	}
	switch g.intn(n) {
	case 0:
		return g.leaf(t)
	case 1:
		return g.leafLit(t)
	case 2, 3, 4:
		return g.binary(t, d)
	case 5:
		if !t.isUnsigned() {
			e := g.expr(t, d-1)
			return ex{s: "-(" + e.s + ")", c: e.c}
		}
	case 6:
		if t.kind == kInt {
			if vs := g.sliceVars(); len(vs) > 0 {
				return ex{s: [2]string{"len", "cap"}[g.intn(2)] + "(" + vs[g.intn(len(vs))].name + ")"}
			}
			if vs := g.varsOf(tString); len(vs) > 0 {
				return ex{s: "len(" + vs[g.intn(len(vs))].name + ")"}
			}
		}
	case 7:
		if o := g.otherNumeric(t); o != nil {
			e := g.nonconst(o, d-1)
			return ex{s: t.String() + "(" + e.s + ")"}
		}
	case 8:
		a, b := g.expr(t, d-1), g.expr(t, d-1)
		return ex{s: [2]string{"min", "max"}[g.intn(2)] + "(" + a.s + ", " + b.s + ")", c: a.c && b.c}
	case 10, 11, 12:
		return g.shiftExpr(t, d)
	}
	if e, ok := g.access(t, d); ok {
		return e
	}
	return g.leaf(t)
}

func (g *gen) otherNumeric(*typ) *typ {
	c := []*typ{tInt, tInt64, tInt32, tUint8, tUint, tFloat}
	return c[g.intn(len(c))]
}

func (g *gen) sliceVars() []*variable {
	var out []*variable
	for _, v := range g.allVars() {
		if v.t.kind == kSlice {
			out = append(out, v)
		}
	}
	return out
}

func (g *gen) binary(t *typ, d int) ex {
	a, b := g.expr(t, d-1), g.expr(t, d-1)
	if a.c && b.c {
		a = g.scalarVar(t)
	}
	var ops []string
	switch {
	case t.isString():
		ops = []string{"+"}
	case t.under().kind == kFloat:
		ops = []string{"+", "-", "*", "/"}
	default:
		ops = []string{"+", "-", "*", "/", "%", "&", "|", "^", "&^"}
	}
	op := ops[g.intn(len(ops))]
	switch op {
	case "/", "%":
		if t.isInteger() {
			b = ex{s: "(" + b.s + " | 1)", c: b.c}
		} else {
			b = ex{s: "(" + b.s + " + 1.5)", c: b.c}
		}
	}
	return ex{s: "(" + a.s + " " + op + " " + b.s + ")", c: a.c && b.c}
}

func (g *gen) strExpr(t *typ, d int) ex {
	n := 8
	if g.o.Extras {
		n = 10
	}
	switch g.intn(n) {
	case 8, 9:
		if t.kind == kString {
			return ex{s: "string(" + g.nonconst(tInt32, d-1).s + ")"} // a rune: no vet warning
		}
	case 0:
		return g.leaf(t)
	case 1:
		return g.leafLit(t)
	case 2, 3:
		return g.binary(t, d)
	case 4:
		if t.kind == kString && !g.static {
			e := g.expr(g.scalarTypes()[g.intn(len(g.scalarTypes()))], d-1)
			return ex{s: "fmt.Sprint(" + e.s + ")"}
		}
	case 5:
		if t.kind == kString && !g.static {
			a, b := g.expr(tInt, d-1), g.expr(tString, d-1)
			return ex{s: "fmt.Sprintf(\"%d:%s\", " + a.s + ", " + b.s + ")"}
		}
	case 6:
		if t.kind == kString {
			e := g.expr(&typ{kind: kSlice, elem: tUint8}, d-1)
			return ex{s: "string(" + e.s + ")"}
		}
		e := g.nonconst(tString, d-1)
		return ex{s: t.String() + "(" + e.s + ")"}
	case 7:
		if t.kind == kString {
			s := g.nonconst(tString, d-1)
			return ex{s: s.s + "[" + g.maybeIdx(d) + ":" + g.maybeIdx(d) + "]"}
		}
	}
	if e, ok := g.access(t, d); ok {
		return e
	}
	return g.leaf(t)
}

// maybeIdx returns an index for a slice expression: nothing, or a variable expression. (Two
// constants could be out of order.)
func (g *gen) maybeIdx(d int) string {
	if g.chance(2) {
		return ""
	}
	return g.nonconst(tInt, d-1).s
}

func (g *gen) scalarTypes() []*typ {
	return append(append([]*typ{}, basics...), g.named...)
}

func (g *gen) boolExpr(t *typ, d int) ex {
	switch g.intn(9) {
	case 0:
		return g.leaf(t)
	case 1:
		return g.leafLit(t)
	case 2:
		e := g.expr(t, d-1)
		return ex{s: "!" + wrap(e.s), c: e.c}
	case 3:
		a, b := g.expr(t, d-1), g.expr(t, d-1)
		return ex{s: "(" + a.s + " " + [2]string{"&&", "||"}[g.intn(2)] + " " + b.s + ")", c: a.c && b.c}
	case 4, 5, 6:
		ts := g.scalarTypes()
		s := ts[g.intn(len(ts))]
		ops := []string{"==", "!="}
		if s.isOrdered() {
			ops = []string{"==", "!=", "<", "<=", ">", ">="}
		}
		a, b := g.expr(s, d-1), g.expr(s, d-1)
		return ex{s: "(" + a.s + " " + ops[g.intn(len(ops))] + " " + b.s + ")", c: a.c && b.c}
	case 7:
		// Compared with nil, or two values of a comparable type.
		var c []*variable
		for _, v := range g.allVars() {
			if v.t.nilable() && v.t.kind != kAny {
				c = append(c, v)
			}
		}
		if len(c) > 0 {
			return ex{s: "(" + c[g.intn(len(c))].name + " " + [2]string{"==", "!="}[g.intn(2)] + " nil)"}
		}
	}
	if e, ok := g.access(t, d); ok {
		return e
	}
	return g.leaf(t)
}

func wrap(s string) string {
	if strings.HasPrefix(s, "(") || !strings.ContainsAny(s, " ") {
		return s
	}
	return "(" + s + ")"
}

// compExpr is expr for the types that are not scalar.
func (g *gen) compExpr(t *typ, d int) ex {
	if g.chance(3) {
		if e, ok := g.access(t, d); ok {
			return e
		}
	}
	switch t.kind {
	case kSlice:
		switch g.intn(7) {
		case 6:
			if g.o.Extras {
				if t.elem.kind == kInt32 && g.chance(2) {
					return ex{s: "[]int32(" + g.nonconst(tString, d-1).s + ")"}
				}
				if e, ok := g.sliceExpr3(t, d); ok {
					return e
				}
			}
		case 0:
			return g.leaf(t)
		case 1, 2:
			return ex{s: t.String() + "{" + g.elems(t.elem, g.intn(4), d) + "}"}
		case 3:
			if !g.static {
				return ex{s: "make(" + t.String() + ", " + g.nonconst(tInt, d-1).s + ")"}
			}
		case 4:
			if !g.static {
				a := g.expr(t, d-1)
				if g.chance(3) {
					return ex{s: "append(" + a.s + ", " + g.expr(t, d-1).s + "...)"}
				}
				return ex{s: "append(" + a.s + ", " + g.elems(t.elem, 1+g.intn(3), d) + ")"}
			}
		case 5:
			if t.elem.kind == kUint8 {
				return ex{s: "[]uint8(" + g.nonconst(tString, d-1).s + ")"}
			}
			if vs := g.varsOf(t); len(vs) > 0 {
				return ex{s: vs[g.intn(len(vs))].name + "[" + g.maybeIdx(d) + ":" + g.maybeIdx(d) + "]"}
			}
		}
	case kArray:
		return ex{s: t.String() + "{" + g.elems(t.elem, g.intn(t.n+1), d) + "}"}
	case kMap:
		switch g.intn(4) {
		case 0:
			return g.leaf(t)
		case 1:
			if !g.static {
				return ex{s: "make(" + t.String() + ")"}
			}
		}
		return g.mapLit(t, d)
	case kPtr:
		switch g.intn(4) {
		case 0:
			return g.leaf(t)
		case 1:
			if vs := g.varsOf(t.elem); len(vs) > 0 && !g.static {
				return ex{s: "&" + vs[g.intn(len(vs))].name}
			}
		case 2:
			return ex{s: "new(" + t.elem.String() + ")"}
		}
		if t.elem.kind == kStruct {
			return ex{s: "&" + g.structLit(t.elem, d).s}
		}
		return ex{s: "new(" + t.elem.String() + ")"}
	case kStruct:
		if g.chance(4) {
			return g.leaf(t)
		}
		return g.structLit(t, d)
	case kChan:
		if g.chance(3) {
			return g.leaf(t)
		}
		if g.chance(2) {
			return ex{s: "make(" + t.String() + ")"}
		}
		return ex{s: "make(" + t.String() + ", " + fmt.Sprint(g.intn(4)) + ")"}
	case kFunc:
		if g.static || g.chance(4) {
			return g.leaf(t)
		}
		return g.funcLit(t, d)
	case kIface:
		if g.chance(4) || len(t.impls) == 0 {
			return g.leaf(t)
		}
		s := t.impls[g.intn(len(t.impls))]
		return ex{s: "&" + g.structLit(s, d).s}
	case kAny:
		if g.chance(2) {
			return g.leaf(t)
		}
		ts := append(g.scalarTypes(), &typ{kind: kSlice, elem: tInt})
		o := ts[g.intn(len(ts))]
		e := g.expr(o, d-1)
		if e.c && o.kind != kNamed && o.kind != kSlice && g.chance(2) {
			return ex{s: o.String() + "(" + e.s + ")"} // 3 is an int in an any, int64(3) is not
		}
		return e
	}
	return g.leaf(t)
}

func (g *gen) elems(t *typ, n, d int) string {
	var parts []string
	for i := 0; i < n; i++ {
		parts = append(parts, g.expr(t, d-1).s)
	}
	return strings.Join(parts, ", ")
}

func (g *gen) mapLit(t *typ, d int) ex {
	k := t.key
	var parts []string
	n := g.intn(3)
	for i := 0; i < n; i++ {
		var key string
		switch {
		case k.isString():
			key = fmt.Sprintf("%q", fmt.Sprintf("k%d", i))
			if k.kind == kNamed {
				key = k.name + "(" + key + ")"
			}
		case k.isInteger():
			key = fmt.Sprint(10 + i)
			if k.kind == kNamed {
				key = k.name + "(" + key + ")"
			}
		default:
			n = 0 // a key of another type: one entry at most is not worth the risk of a duplicate
		}
		if key == "" {
			break
		}
		parts = append(parts, key+": "+g.expr(t.elem, d-1).s)
	}
	return ex{s: t.String() + "{" + strings.Join(parts, ", ") + "}"}
}

func (g *gen) structLit(t *typ, d int) ex {
	var parts []string
	for _, f := range t.fields {
		if f.embedded {
			if g.chance(3) {
				parts = append(parts, f.name+": "+g.structLit(f.t, d-1).s)
			}
			continue
		}
		if g.chance(2) {
			parts = append(parts, f.name+": "+g.expr(f.t, d-1).s)
		}
	}
	return ex{s: t.name + "{" + strings.Join(parts, ", ") + "}"}
}

// funcLit returns a function literal of the type ft (its body is written by block).
func (g *gen) funcLit(ft *typ, d int) ex {
	var sb strings.Builder
	saved := g.out
	g.out = strings.Builder{}
	res, loops, breaks, labels := g.res, g.loops, g.breaks, g.labels
	g.res, g.loops, g.breaks, g.labels = ft.results, 0, 0, nil
	g.push()
	var ps []string
	for _, p := range ft.params {
		n := g.name("p")
		ps = append(ps, n+" "+p.String())
		g.declare(n, p)
	}
	g.ind++
	g.block(d-1, 1+g.intn(3))
	g.ret()
	g.ind--
	g.pop()
	body := g.out.String()
	g.out = saved
	g.res, g.loops, g.breaks, g.labels = res, loops, breaks, labels
	sb.WriteString("func(" + strings.Join(ps, ", ") + ")" + resultList(ft.results) + " {\n")
	sb.WriteString(body)
	sb.WriteString(strings.Repeat("\t", g.ind) + "}")
	return ex{s: sb.String()}
}

// access returns an expression of type t that reads a variable: the variable itself, an element
// of a slice, array or map, a field, a pointer's target, a received value, or the result of a
// call of a function or method with the one result t.
func (g *gen) access(t *typ, d int) (ex, bool) {
	var c []func() ex
	ts := t.String()
	add := func(f func() ex) { c = append(c, f) }
	for _, v := range g.allVars() {
		v := v
		vt := v.t
		if vt.String() == ts {
			add(func() ex { return ex{s: v.name} })
		}
		switch vt.kind {
		case kSlice:
			if vt.elem.String() == ts {
				add(func() ex { return ex{s: v.name + "[" + g.idx(d-1) + "]"} })
			}
		case kArray:
			if vt.elem.String() == ts {
				add(func() ex {
					if g.chance(2) {
						return ex{s: fmt.Sprintf("%s[%d]", v.name, g.intn(vt.n))}
					}
					return ex{s: v.name + "[" + g.nonconst(tInt, d-1).s + "]"}
				})
			}
		case kMap:
			if vt.elem.String() == ts {
				add(func() ex { return ex{s: v.name + "[" + g.expr(vt.key, d-1).s + "]"} })
			}
		case kPtr:
			if vt.elem.String() == ts {
				add(func() ex { return ex{s: "*" + v.name} })
			}
		case kChan:
			if vt.elem.String() == ts && !g.static {
				add(func() ex { return ex{s: "<-" + v.name} })
			}
		}
		st := vt
		if vt.kind == kPtr {
			st = vt.elem
		}
		if st.kind == kStruct {
			for _, m := range st.allFields() {
				m := m
				if m.f.t.String() == ts {
					add(func() ex { return ex{s: v.name + "." + m.path + m.f.name} })
				}
			}
		}
		if !g.static {
			if vt.kind == kFunc && len(vt.results) == 1 && vt.results[0].String() == ts {
				add(func() ex { return ex{s: v.name + "(" + g.args(vt.params, d) + ")"} })
			}
			recv := vt
			if recv.kind == kPtr {
				recv = recv.elem
			}
			if recv.kind == kStruct || recv.kind == kNamed {
				for _, m := range recv.allMethods() {
					m := m
					if len(m.m.results) == 1 && m.m.results[0].String() == ts {
						add(func() ex { return ex{s: v.name + "." + m.path + m.m.name + "(" + g.args(m.m.params, d) + ")"} })
					}
					if t.kind == kFunc && sameSig(m.m.params, t.params) && sameSig(m.m.results, t.results) && (recv.kind == kStruct || !m.m.ptr) {
						add(func() ex { return ex{s: v.name + "." + m.path + m.m.name} })
					}
				}
			}
			if vt.kind == kIface {
				for _, m := range vt.methods {
					m := m
					if len(m.results) == 1 && m.results[0].String() == ts {
						add(func() ex { return ex{s: v.name + "." + m.name + "(" + g.args(m.params, d) + ")"} })
					}
				}
			}
		}
		if vt.kind == kAny || vt.kind == kIface {
			// A type assertion to t: any value of any type; an interface to a pointer of a
			// struct that implements it.
			if vt.kind == kAny && t.kind != kAny && t.kind != kIface && !g.static {
				add(func() ex { return ex{s: v.name + ".(" + ts + ")"} })
			}
			if vt.kind == kIface && t.kind == kPtr && !g.static {
				for _, s := range vt.impls {
					if s == t.elem {
						add(func() ex { return ex{s: v.name + ".(" + ts + ")"} })
					}
				}
			}
		}
	}
	if !g.static {
		for _, f := range g.funcs {
			f := f
			if len(f.results) == 1 && f.results[0].String() == ts {
				add(func() ex { return ex{s: f.name + "(" + g.callArgs(f, d) + ")"} })
			}
			if t.kind == kFunc && !f.variadic && sameSig(f.params, t.params) && sameSig(f.results, t.results) {
				add(func() ex { return ex{s: f.name} })
			}
		}
		if e, ok := g.genericCall(t, d); ok {
			add(func() ex { return e })
		}
	}
	if len(c) == 0 {
		return ex{}, false
	}
	return c[g.intn(len(c))](), true
}

// args returns the arguments for the parameter types.
func (g *gen) args(ps []*typ, d int) string {
	var parts []string
	for _, p := range ps {
		parts = append(parts, g.expr(p, d-1).s)
	}
	return strings.Join(parts, ", ")
}

// callArgs returns the arguments for a call of the function f, with extra ones for a variadic function.
func (g *gen) callArgs(f *fn, d int) string {
	if !f.variadic {
		return g.args(f.params, d)
	}
	n := len(f.params)
	s := g.args(f.params[:n-1], d)
	last := f.params[n-1]
	var rest string
	if g.chance(3) {
		rest = g.expr(last, d-1).s + "..."
	} else {
		rest = g.elems(last.elem, g.intn(3), d)
	}
	if s != "" && rest != "" {
		return s + ", " + rest
	}
	return s + rest
}
