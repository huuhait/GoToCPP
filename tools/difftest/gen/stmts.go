package gen

import (
	"fmt"
	"strconv"
	"strings"
)

// A soup statement is one random statement over the variables in scope. Scenarios prepare
// variables of the types they care about and then run the soup, or write their own code.

type soupStmt struct {
	name  string
	w     int
	depth int // needs at least this much nesting budget
	need  []string
	fn    func(g *Gen, depth int)
}

var soup []soupStmt

func registerStmt(name string, w, depth int, need []string, fn func(g *Gen, depth int)) {
	soup = append(soup, soupStmt{name: name, w: w, depth: depth, need: need, fn: fn})
}

func (g *Gen) stmts(n, depth int) {
	for i := 0; i < n; i++ {
		g.stmt(depth)
	}
}

func (g *Gen) stmt(depth int) {
	var ws []weighted
	for _, s := range soup {
		if s.depth > depth {
			continue
		}
		ok := true
		for _, n := range s.need {
			if !g.f[n] {
				ok = false
			}
		}
		if !ok {
			continue
		}
		s := s
		ws = append(ws, weighted{s.w, s.name, func() { s.fn(g, depth) }})
	}
	g.choose(ws)
}

// mid ends the current block and starts the next part of the statement ("} else {").
func (g *Gen) mid(text string) {
	g.sweep()
	g.popScope()
	g.ind--
	g.ln("%s", text)
	g.ind++
	g.pushScope()
}

// leave ends the block opened by enter.
func (g *Gen) leave() { g.close() }

// ---- printing ----

// show prints the value e of type t with a random verb.
func (g *Gen) show(e string, t *Type) {
	label := strconv.Quote(g.fresh("o") + ":")
	switch t.K {
	case KInt, KFloat, KBool, KString:
		switch g.roll(50, 45, 5) {
		case 0:
			g.ln("fmt.Println(%s, %s)", label, e)
		case 1:
			g.ln("fmt.Printf(%s, %s)", strconv.Quote("%s "+g.verbFor(t)+"\n"), "\"o\", "+e)
		default:
			g.ln("fmt.Printf(%s, %s)", strconv.Quote("%v %T\n"), e+", "+e)
		}
		return
	}
	if !t.NoPtr && t.K != KPtr {
		g.ln("fmt.Println(%s, %s)", label, e)
		return
	}
	switch g.roll(30, 25, 25, 20) {
	case 0:
		g.ln("fmt.Println(%s, %s)", label, e)
	case 1:
		g.ln("fmt.Printf(\"%%+v\\n\", %s)", e)
	case 2:
		g.ln("fmt.Printf(\"%%#v\\n\", %s)", e)
	default:
		g.ln("fmt.Printf(\"%%v %%T\\n\", %s, %s)", e, e)
	}
}

// showVar prints a variable.
func (g *Gen) showVar(v *Var) {
	v.Used = true
	if v.T.K == KPtr {
		if v.NonNil {
			switch v.T.Elem.K {
			case KStruct, KArray, KSlice, KMap:
				g.show(v.Name, v.T) // &{...}
			default:
				g.show("*"+v.Name, v.T.Elem)
			}
		}
		return
	}
	if v.T.K == KIface || v.T.K == KFunc || v.T.K == KChan {
		return
	}
	g.show(v.Name, v.T)
}

// showAll prints every variable in scope (the innermost scope first), to make the
// scenario's final state visible.
func (g *Gen) showAll() {
	for i := len(g.scps) - 1; i >= 0; i-- {
		for _, v := range g.scps[i].vars {
			if !v.NoRead {
				g.showVar(v)
			}
		}
	}
}

// ---- setup helpers ----

// seedVars declares n variables of random types.
func (g *Gen) seedVars(n int) {
	for i := 0; i < n; i++ {
		g.declareVal("v", g.randType(2))
	}
}

// seedBasic declares n variables of random basic types, at least some of them integers.
func (g *Gen) seedBasic(n int) {
	for i := 0; i < n; i++ {
		var t *Type
		if i < 2 || g.chance(60) {
			t = g.randIntType()
		} else {
			t = g.randBasic()
		}
		g.declareVal("v", t)
	}
}

// ---- soup statements ----

func init() {
	registerStmt("assign", 26, 0, nil, stAssign)
	registerStmt("compound", 26, 0, nil, stCompound)
	registerStmt("incdec", 6, 0, nil, stIncDec)
	registerStmt("declare", 8, 0, nil, stDeclare)
	registerStmt("print", 12, 0, nil, stPrint)
	registerStmt("if", 9, 1, nil, stIf)
	registerStmt("for", 6, 1, nil, stFor)
	registerStmt("switch", 5, 1, nil, stSwitch)
	registerStmt("rangeloop", 5, 1, nil, stRange)
	registerStmt("rangeint", 3, 1, []string{"ints"}, stRangeInt)
	registerStmt("shadow", 4, 1, []string{"shadow"}, stShadow)
	registerStmt("slice", 7, 0, []string{"slices"}, stSliceOp)
	registerStmt("map", 6, 0, []string{"maps"}, stMapOp)
	registerStmt("copyval", 4, 0, []string{"structs"}, stCopyVal)
	registerStmt("block", 2, 1, nil, stBlock)
}

func stAssign(g *Gen, d int) {
	t := g.randBasic()
	ps := g.writable(t)
	if len(ps) == 0 {
		g.declareVal("v", t)
		return
	}
	p := pick(g, ps)
	e := g.expr(t, 2)
	if t.K == KString {
		e = "clipS(" + e + ")" // strings must not grow without bound in nested loops
		if p.E == p.Root.Name && p.Root.MinLen > 0 {
			if g.inLoop() {
				g.ln("%s = clipS(%s + %s)", p.E, p.E, e) // keeps the length that the other code relies on
				return
			}
			p.Root.MinLen = 0
		}
	}
	g.ln("%s = %s", p.E, e)
}

func stCompound(g *Gen, d int) {
	t := g.randBasic()
	ps := g.writable(t)
	if len(ps) == 0 {
		g.declareVal("v", t)
		return
	}
	p := pick(g, ps)
	switch t.K {
	case KInt:
		switch g.roll(60, 12, 14, 6) {
		case 0:
			e := g.expr(t, 1)
			g.ln("%s %s= %s", p.E, pick(g, []string{"+", "-", "*", "&", "|", "^", "&^"}), e)
		case 1:
			e := g.expr(t, 1)
			g.ln("%s %s= (%s | 1)", p.E, pick(g, []string{"/", "%"}), e)
		case 2:
			g.ln("%s %s= %s", p.E, pick(g, []string{"<<", ">>"}), g.shiftCount(t.Bits))
		default:
			raw := g.intRaw(t)
			for raw == "0" {
				raw = g.intRaw(t)
			}
			g.ln("%s %s= %s", p.E, pick(g, []string{"/", "%"}), paren(raw))
		}
	case KFloat:
		g.ln("%s %s= %s", p.E, pick(g, []string{"+", "-", "*"}), g.expr(t, 1))
	case KString:
		g.ln("%s = clipS(%s + %s)", p.E, p.E, par(g.expr(t, 1)))
	case KBool:
		g.ln("%s = %s %s %s", p.E, p.E, pick(g, []string{"&&", "||"}), par(g.boolExpr(1)))
	}
}

func stIncDec(g *Gen, d int) {
	t := g.randIntType()
	ps := g.writable(t)
	if len(ps) == 0 {
		g.declareVal("v", t)
		return
	}
	g.ln("%s%s", pick(g, ps).E, pick(g, []string{"++", "--"}))
}

func stDeclare(g *Gen, d int) {
	g.declareVal("v", g.randType(2))
}

func stPrint(g *Gen, d int) {
	t := g.randType(2)
	var vs []*Var
	for _, v := range g.allVars() {
		if v.T.Name == t.Name {
			vs = append(vs, v)
		}
	}
	if len(vs) > 0 && g.chance(60) {
		g.showVar(pick(g, vs))
		return
	}
	g.show(g.val(t, 2), t)
}

func stIf(g *Gen, d int) {
	c := g.boolExpr(2)
	switch g.roll(60, 25, 15) {
	case 0:
		g.enter("if " + c)
		g.stmts(g.between(1, 3), d-1)
		g.leave()
	case 1:
		g.enter("if " + c)
		g.stmts(g.between(1, 3), d-1)
		g.mid("} else {")
		g.stmts(g.between(1, 3), d-1)
		g.leave()
	default:
		t := g.randIntType()
		name := g.fresh("w")
		init := g.expr(t, 1)
		g.enter(fmt.Sprintf("if %s := %s; %s %s %s", name, init, name, pick(g, []string{"<", ">", "==", "!=", "<=", ">="}), paren(g.intRaw(t))))
		g.bind(name, t)
		g.stmts(g.between(1, 2), d-1)
		g.mid("} else if " + name + " != 0 {")
		g.stmts(g.between(1, 2), d-1)
		g.mid("} else {")
		g.stmts(g.between(1, 2), d-1)
		g.leave()
	}
}

func stFor(g *Gen, d int) {
	name := g.fresh("i")
	n := g.between(0, 5)
	switch g.roll(60, 25, 15) {
	case 0:
		g.enter(fmt.Sprintf("for %s := 0; %s < %d; %s++", name, name, n, name))
		g.bindRO(name, intTs["int"])
		g.stmts(g.between(1, 3), d-1)
		g.leave()
	case 1: // down-counting with a break and continue
		g.enter(fmt.Sprintf("for %s := %d; %s > 0; %s--", name, n, name, name))
		g.bindRO(name, intTs["int"])
		g.enter("if " + g.boolExpr(1))
		g.ln("continue")
		g.leave()
		g.stmts(g.between(1, 2), d-1)
		g.enter(fmt.Sprintf("if %s == %d", name, g.between(1, 3)))
		g.ln("break")
		g.leave()
		g.leave()
	default: // while-style with a counter
		c := g.fresh("c")
		g.ln("%s := 0", c)
		g.enter(fmt.Sprintf("for %s < %d", c, n))
		g.ln("%s++", c)
		g.stmts(g.between(1, 2), d-1)
		g.leave()
		g.ln("_ = %s", c)
	}
}

func stRangeInt(g *Gen, d int) {
	name := g.fresh("k")
	g.enter(fmt.Sprintf("for %s := range %d", name, g.between(0, 5)))
	g.bindRO(name, intTs["int"])
	g.stmts(g.between(1, 3), d-1)
	g.leave()
}

func stRange(g *Gen, d int) {
	var cands []*Var
	for _, v := range g.allVars() {
		switch v.T.K {
		case KSlice, KArray, KString:
			cands = append(cands, v)
		}
	}
	if len(cands) == 0 {
		return
	}
	v := pick(g, cands)
	v.Used = true
	i, e := g.fresh("i"), g.fresh("e")
	switch {
	case v.T.K == KString:
		g.enter(fmt.Sprintf("for %s, %s := range %s", i, e, v.Name))
		g.bindRO(i, intTs["int"])
		g.bind(e, intTs["int32"])
	case g.chance(25):
		g.enter(fmt.Sprintf("for %s := range %s", i, v.Name))
		g.bindRO(i, intTs["int"])
	default:
		g.enter(fmt.Sprintf("for %s, %s := range %s", i, e, v.Name))
		g.bindRO(i, intTs["int"])
		g.bind(e, v.T.Elem)
	}
	g.stmts(g.between(1, 3), d-1)
	g.leave()
}

func stShadow(g *Gen, d int) {
	t := g.randIntType()
	if g.chance(30) {
		t = tBool
	}
	vs := g.varsOf(t)
	if len(vs) == 0 {
		return
	}
	v := pick(g, vs)
	v.Used = true
	g.ln("{")
	g.ind++
	g.pushScope()
	switch t.K {
	case KInt:
		g.ln("%s := %s %s %s", v.Name, v.Name, pick(g, []string{"+", "*", "^", "-"}), g.expr(t, 1))
	default:
		g.ln("%s := !%s", v.Name, v.Name)
	}
	g.add(&Var{Name: v.Name, T: t})
	g.stmts(g.between(1, 3), d-1)
	g.showVar(g.scps[len(g.scps)-1].vars[0])
	g.close()
	g.showVar(v)
}

func stBlock(g *Gen, d int) {
	g.ln("{")
	g.ind++
	g.pushScope()
	g.stmts(g.between(1, 3), d-1)
	g.close()
}

func stSwitch(g *Gen, d int) {
	t := g.randIntType()
	tag := g.nc(t, 1)
	n := g.between(1, 4)
	g.ln("switch %s {", tag)
	seen := map[string]bool{}
	fall := false
	for i := 0; i < n; i++ {
		var ks []string
		for j := g.between(1, 3); j > 0; j-- {
			k := paren(g.smallConst(t))
			if !seen[k] {
				seen[k] = true
				ks = append(ks, k)
			}
		}
		if len(ks) == 0 {
			continue
		}
		if fall {
			g.ln("\tfallthrough")
		}
		g.ln("case %s:", strings.Join(ks, ", "))
		g.ind++
		g.pushScope()
		g.stmts(g.between(1, 2), d-1)
		g.sweep()
		g.popScope()
		g.ind--
		fall = g.chance(30)
		if g.chance(15) {
			if fall {
				g.ln("\tfallthrough")
			}
			g.ln("default:")
			g.ind++
			g.pushScope()
			g.stmts(1, d-1)
			g.sweep()
			g.popScope()
			g.ind--
			// only one default
			g.ln("}")
			return
		}
	}
	g.ln("}")
}

// smallConst returns a small constant of type t, usable as a case.
func (g *Gen) smallConst(t *Type) string {
	if t.Signed {
		return strconv.Itoa(g.between(-3, 4))
	}
	return strconv.Itoa(g.between(0, 6))
}

func stSliceOp(g *Gen, d int) {
	var cands []*Var
	for _, v := range g.allVars() {
		if v.T.K == KSlice {
			cands = append(cands, v)
		}
	}
	if len(cands) == 0 {
		return
	}
	v := pick(g, cands)
	switch g.roll(40, 15, 15, 10, 10, 10) {
	case 0: // append one or more
		n := g.between(1, 3)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = g.val(v.T.Elem, 1)
		}
		g.ln("%s = append(%s, %s)", v.Name, v.Name, strings.Join(parts, ", "))
		g.ln("if len(%s) > 32 { %s = %s[:32] }", v.Name, v.Name, v.Name) // slices must not grow without bound in nested loops
		g.grow(v, n)
		v.MinLen = min(v.MinLen, 32)
	case 1: // append a slice
		var os []*Var
		for _, o := range g.allVars() {
			if o.T.Name == v.T.Name {
				os = append(os, o)
			}
		}
		o := pick(g, os)
		o.Used = true
		g.ln("%s = append(%s, %s[:len(%s)%%3]...)", v.Name, v.Name, o.Name, o.Name) // at most two: no doubling in loops
		g.ln("if len(%s) > 32 { %s = %s[:32] }", v.Name, v.Name, v.Name)
		v.MinLen = min(v.MinLen, 32)
	case 2: // set an element
		if v.MinLen > 0 {
			g.ln("%s[%s] = %s", v.Name, g.idxDyn(v.Name), g.val(v.T.Elem, 1))
		}
	case 3: // sub-slice the front off
		if v.MinLen > 1 && !g.inLoop() {
			k := g.between(1, v.MinLen)
			g.ln("%s = %s[%d:]", v.Name, v.Name, k)
			v.MinLen -= k
		}
	case 4: // copy
		var os []*Var
		for _, o := range g.allVars() {
			if o.T.Name == v.T.Name && o != v {
				os = append(os, o)
			}
		}
		if len(os) > 0 {
			o := pick(g, os)
			o.Used = true
			g.ln("fmt.Println(\"copy\", copy(%s, %s))", v.Name, o.Name)
		}
	case 5: // reslice a window into a new variable, which aliases
		if v.MinLen > 1 {
			lo := g.between(0, v.MinLen-1)
			hi := g.between(lo, v.MinLen)
			nv := g.declare("w", v.T, fmt.Sprintf("%s[%d:%d]", v.Name, lo, hi))
			nv.MinLen = hi - lo
			if nv.MinLen > 0 && g.chance(60) {
				g.ln("%s[0] = %s", nv.Name, g.val(v.T.Elem, 1))
			}
			g.ln("fmt.Println(\"len\", len(%s), len(%s))", nv.Name, v.Name)
		}
	}
}

func stMapOp(g *Gen, d int) {
	var cands []*Var
	for _, v := range g.allVars() {
		if v.T.K == KMap && v.NonNil { // a map read from another map may be nil
			cands = append(cands, v)
		}
	}
	if len(cands) == 0 {
		return
	}
	v := pick(g, cands)
	v.Used = true
	k := v.T.Key
	switch g.roll(40, 15, 25, 20) {
	case 0:
		g.ln("%s[%s] = %s", v.Name, g.keyLit(k, g.rn(distinctKeyCount(k))), g.val(v.T.Elem, 1))
	case 1:
		g.ln("delete(%s, %s)", v.Name, g.keyLit(k, g.rn(distinctKeyCount(k))))
	case 2:
		ok, e := g.fresh("ok"), g.fresh("m")
		g.ln("%s, %s := %s[%s]", e, ok, v.Name, g.keyLit(k, g.rn(distinctKeyCount(k))))
		g.bind(e, v.T.Elem).Used = true
		g.bind(ok, tBool).Used = true
		g.show(e, v.T.Elem)
		g.ln("fmt.Println(%s)", ok)
	default:
		g.ln("fmt.Println(len(%s), %s)", v.Name, v.Name)
	}
}

func stCopyVal(g *Gen, d int) {
	var cands []*Var
	for _, v := range g.allVars() {
		if v.T.K == KStruct || v.T.K == KArray {
			cands = append(cands, v)
		}
	}
	if len(cands) == 0 {
		return
	}
	v := pick(g, cands)
	v.Used = true
	c := g.declare("cp", v.T, v.Name)
	// change the copy through some basic path, show both
	var ps []Path
	g.walk(c, c.Name, c.T, 0, &ps)
	var basics []Path
	for _, p := range ps {
		if p.T.isBasic() {
			basics = append(basics, p)
		}
	}
	if len(basics) > 0 {
		p := pick(g, basics)
		g.ln("%s = %s", p.E, g.expr(p.T, 1))
	}
	g.showVar(c)
	g.showVar(v)
	if v.T.comparable() {
		g.ln("fmt.Println(%s == %s)", c.Name, v.Name)
	}
}
