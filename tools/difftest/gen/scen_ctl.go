package gen

import (
	"fmt"
	"strconv"
	"strings"
)

// Scenarios about control flow that is not plain structured code: closures and the Go 1.22
// loop variables, defer, panic and recover, the run-time panics of the language (each made
// by exactly one operation, so the message is the same in every implementation), and the
// panics that end a program.

func init() {
	register("closures", 10, []string{"closures"}, scenClosures)
	register("defer", 12, []string{"defer"}, scenDefer)
	register("rtpanic", 11, []string{"defer"}, scenRuntimePanics)
	register("switchfall", 6, nil, scenSwitchFall)
}

func scenClosures(g *Gen) {
	t := g.randIntType()
	n := g.between(2, 5)
	// Go 1.22: every iteration has its own copy of the loop variable.
	fs := g.fresh("fs")
	g.ln("var %s []func() %s", fs, t.Name)
	g.enter(fmt.Sprintf("for i := 0; i < %d; i++", n))
	g.ln("j := %s(i) * %s", t.Name, paren(g.intRaw(t)))
	g.ln("%s = append(%s, func() %s { i += 10; j++; return %s(i) + j })", fs, fs, t.Name, t.Name)
	g.leave()
	g.enter(fmt.Sprintf("for k, f := range %s", fs))
	g.ln("fmt.Println(k, f(), f())")
	g.leave()
	// range loops: the variables are per iteration too
	g.ln("var gs []func() string")
	g.enter("for i, s := range []string{\"x\", \"yy\", \"z\"}")
	g.ln("gs = append(gs, func() string { return fmt.Sprint(i, s) })")
	g.ln("s += \"!\"")
	g.leave()
	g.enter("for _, f := range gs")
	g.ln("fmt.Println(f())")
	g.leave()
	// counters that share captured state
	g.ln("mk := func(step %s) (func() %s, func() %s) {", t.Name, t.Name, t.Name)
	g.ln("\tc := %s(0)", t.Name)
	g.ln("\treturn func() %s { c += step; return c }, func() %s { c = c * 2; return c }", t.Name, t.Name)
	g.ln("}")
	g.ln("inc, dbl := mk(%s)", g.lit(t))
	g.ln("inc2, _ := mk(%s)", g.lit(t))
	// call order of function values inside one argument list is left to right
	g.ln("fmt.Println(inc(), dbl(), inc(), inc2(), dbl(), inc2())")
	// a closure that assigns to a variable of its enclosing function
	v := g.declareVal("cv", t)
	g.ln("setv := func(x %s) { %s = x }", t.Name, v.Name)
	g.ln("setv(%s)", g.expr(t, 2))
	g.ln("fmt.Println(%s)", v.Name)
	// recursion through a variable
	if g.chance(70) {
		g.ln("var fib func(n int) int")
		g.ln("fib = func(n int) int { if n < 2 { return n }; return fib(n-1) + fib(n-2) }")
		g.ln("fmt.Println(fib(%d))", g.between(5, 14))
		g.ln("fib = nil // breaks the reference cycle that reference counting would leak (D1)")
	}
	// immediately invoked closures and closures as arguments
	g.ln("apply := func(f func(%s) %s, x %s) %s { return f(x) }", t.Name, t.Name, t.Name, t.Name)
	g.ln("fmt.Println(apply(func(x %s) %s { return x*2 + %s }, %s))", t.Name, t.Name, g.lit(t), g.lit(t))
	// method value of a function-typed variable and nil function comparison
	g.ln("var nf func()")
	g.ln("fmt.Println(nf == nil, apply != nil)")
	g.stmts(g.between(0, 4), 1)
	g.showAll()
}

// panicValue returns an expression for a panic value and how fmt prints it after recover.
func (g *Gen) panicValue() string {
	switch g.roll(40, 25, 20, 15) {
	case 0:
		return fmt.Sprintf("fmt.Sprintf(\"boom-%%d\", %d)", g.between(0, 99))
	case 1:
		return strconv.Itoa(g.between(-5, 500))
	case 2:
		return fmt.Sprintf("%s{%d, %q}", g.panicErrType(), g.between(0, 9), pick(g, []string{"a", "bb", "é"}))
	}
	return fmt.Sprintf("&%s{%d, %q}", g.panicErrType(), g.between(0, 9), pick(g, []string{"p", "qq"}))
}

// panicErrType declares (once per program) an error type used as a panic value.
func (g *Gen) panicErrType() string {
	name := g.pfx + "PErr"
	if !g.declared[name] {
		g.declared[name] = true
		g.topf("type %s struct {\n\tCode int\n\tMsg  string\n}\n", name)
		g.topf("func (e %s) Error() string { return fmt.Sprintf(\"perr(%%d,%%s)\", e.Code, e.Msg) }\n", name)
	}
	return name
}

// recoverPrinter is the deferred closure that prints what recover returns.
const recoverPrinter = "defer func() { fmt.Println(\"recovered:\", recover()) }()"

func scenDefer(g *Gen) {
	t := g.randIntType()
	// 1. deferred calls run last-in first-out, their arguments are evaluated at the defer
	f1 := g.topName("df")
	g.topFunc(fmt.Sprintf("%s(a %s) (r %s)", f1, t.Name, t.Name), []*Var{{Name: "a", T: t}}, func() {
		g.ln("x := a")
		g.ln("defer fmt.Println(\"defer1 x =\", x, \"r =\", r)")
		g.enter(fmt.Sprintf("for i := 0; i < %d; i++", g.between(2, 4)))
		g.ln("defer fmt.Println(\"loop defer\", i, x)")
		g.ln("x += %s", paren(g.intRaw(t)))
		g.leave()
		g.ln("defer func() { r += %s; fmt.Println(\"closure defer\", x, r) }()", paren(g.intRaw(t)))
		g.ln("x = %s", g.expr(t, 2))
		g.ln("return x")
	})
	g.ln("fmt.Println(%s(%s))", f1, g.lit(t))
	// 2. recover stops a panic and a deferred function can set the result
	f2 := g.topName("dr")
	pv := g.panicValue()
	g.topFunc(fmt.Sprintf("%s(a %s) (r %s, msg string)", f2, t.Name, t.Name), []*Var{{Name: "a", T: t}}, func() {
		g.ln("defer func() {")
		g.ln("\tif e := recover(); e != nil {")
		g.ln("\t\tmsg = fmt.Sprint(\"caught \", e)")
		g.ln("\t\tr = a + 1")
		g.ln("\t}")
		g.ln("}()")
		g.ln("defer fmt.Println(\"runs while panicking\", a)")
		g.ln("if a %% 2 == 0 {")
		g.ln("\tpanic(%s)", pv)
		g.ln("}")
		g.ln("return a * 2, \"fine\"")
	})
	g.ln("fmt.Println(%s(%s))", f2, g.lit(t))
	g.ln("fmt.Println(%s(%s))", f2, "2")
	// 3. recover only works when the deferred function itself calls it
	f3 := g.topName("dn")
	// the helper must not be inlined: whether gc inlines it decides what recover does (D28)
	hn := g.topName("rh")
	g.topf("//go:noinline\nfunc %s() interface{} { return recover() }\n", hn)
	g.topf("func %s() {\n\tdefer func() { fmt.Println(\"outer recover:\", recover()) }()\n\tdefer func() { fmt.Println(\"helper recover:\", %s()) }()\n\tpanic(%s)\n}\n", f3, hn, g.panicValue())
	g.ln("%s()", f3)
	// 4. a panic in a deferred function replaces the one in flight; re-panic
	f4 := g.topName("dp")
	g.topf("func %s() {\n\tdefer func() { fmt.Println(\"final:\", recover()) }()\n\tdefer func() {\n\t\tr := recover()\n\t\tfmt.Println(\"first:\", r)\n\t\tpanic(fmt.Sprint(\"re-\", r))\n\t}()\n\tdefer func() { panic(%s) }()\n\tpanic(%s)\n}\n", f4, g.panicValue(), g.panicValue())
	g.ln("%s()", f4)
	// 5. defer order with method values and defers inside loops of closures
	g.ln("func() {")
	g.ind++
	g.ln("%s", recoverPrinter)
	g.ln("var s []int")
	g.ln("defer func() { fmt.Println(\"s at exit\", s) }()")
	g.enter(fmt.Sprintf("for i := 0; i < %d; i++", g.between(2, 4)))
	g.ln("defer func() { s = append(s, i) }()")
	g.leave()
	g.ln("panic(%s)", g.panicValue())
	g.ind--
	g.ln("}()")
	// 6. the value of recover is nil outside a panic; nested panics
	g.ln("fmt.Println(recover() == nil)")
	g.ln("func() {")
	g.ind++
	g.ln("defer func() { fmt.Println(\"outer\", recover()) }()")
	g.ln("func() {")
	g.ind++
	g.ln("defer func() { fmt.Println(\"inner\", recover()); panic(%s) }()", g.panicValue())
	g.ln("panic(%s)", g.panicValue())
	g.ind--
	g.ln("}()")
	g.ind--
	g.ln("}()")
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

// scenRuntimePanics makes the run-time panics of the language, one per recovered function.
func scenRuntimePanics(g *Gen) {
	t := g.randIntType()
	each := func(body func()) {
		g.ln("func() {")
		g.ind++
		g.pushScope()
		g.ln("%s", recoverPrinter)
		body()
		g.sweep()
		g.popScope()
		g.ind--
		g.ln("}()")
	}
	// integer division by a zero variable, and the one overflowing division that does not panic
	each(func() {
		z := g.declare("z", t, t.Name+"(0)")
		a := g.declareVal("a", t)
		g.ln("fmt.Println(%s / %s)", a.Name, z.Name)
	})
	each(func() {
		z := g.declare("z", t, t.Name+"(0)")
		a := g.declareVal("a", t)
		g.ln("fmt.Println(%s %% %s)", a.Name, z.Name)
	})
	if t.Signed {
		lo, _ := intRange(t)
		g.ln("{")
		g.ind++
		g.ln("m1 := %s(-1)", t.Name)
		g.ln("lo := %s(%s)", t.Name, lo)
		g.ln("fmt.Println(lo/m1, lo%%m1, -lo, lo*m1)")
		g.ind--
		g.ln("}")
	}
	// out of range index and slice bounds with dynamic values
	if g.has("slices") {
		each(func() {
			g.ln("s := make([]%s, %d)", t.Name, g.between(1, 4))
			g.ln("i := len(s) + %d", g.between(0, 3))
			g.ln("fmt.Println(s[i])")
		})
		each(func() {
			g.ln("s := make([]byte, %d, %d)", g.between(1, 3), g.between(4, 6))
			g.ln("hi := len(s) + %d", g.between(3, 8))
			g.ln("fmt.Println(len(s[:hi]))")
		})
		each(func() {
			g.ln("s := make([]int, %d)", g.between(2, 5))
			g.ln("lo, hi := %d, %d", g.between(3, 4), g.between(0, 1))
			g.ln("fmt.Println(len(s[lo:hi]))")
		})
		each(func() {
			g.ln("n := -%d", g.between(1, 3))
			g.ln("fmt.Println(len(make([]int, n)))")
		})
	}
	if g.has("arrays") {
		each(func() {
			g.ln("var a [%d]%s", g.between(2, 5), t.Name)
			g.ln("i := len(a) + %d", g.between(0, 2))
			g.ln("a[i] = 1")
			g.ln("fmt.Println(a)")
		})
	}
	if g.has("strings") {
		each(func() {
			g.ln("s := %s", strconv.Quote(pick(g, []string{"abc", "é", "xy"})))
			g.ln("i := len(s) + %d", g.between(0, 2))
			g.ln("fmt.Println(s[i])")
		})
		each(func() {
			g.ln("s := \"hello\"")
			g.ln("i := %d", g.between(6, 9))
			g.ln("fmt.Println(s[1:i])")
		})
	}
	if g.has("maps") {
		each(func() {
			g.ln("var m map[string]%s", t.Name)
			g.ln("fmt.Println(m[\"a\"], len(m))")
			g.ln("m[\"a\"] = 1")
			g.ln("fmt.Println(m)")
		})
	}
	if g.has("pointers") && g.has("structs") {
		each(func() {
			st := g.newStructWith([]Field{{Name: "A", T: t}, {Name: "B", T: tString}})
			g.ln("var p *%s", st.Name)
			g.ln("fmt.Println(p == nil)")
			g.ln("fmt.Println(p.A)")
		})
		each(func() {
			g.ln("var p *%s", t.Name)
			g.ln("*p = 1")
		})
	}
	// negative shift counts
	each(func() {
		g.ln("n := -%d", g.between(1, 5))
		g.ln("x := %s(1)", t.Name)
		g.ln("fmt.Println(x << n)")
	})
	// failed type assertions
	if g.has("ifaces") {
		each(func() {
			g.ln("var x interface{} = %s", g.lit(tString))
			g.ln("fmt.Println(x.(%s))", t.Name)
		})
		each(func() {
			g.ln("var x interface{}")
			g.ln("fmt.Println(x.(int))")
		})
		each(func() {
			g.ln("var x interface{} = %s(1)", t.Name)
			g.ln("v, ok := x.(string)")
			g.ln("fmt.Printf(\"%%q %%v\\n\", v, ok)")
			g.ln("_ = x.(error)")
		})
	}
	// panic(nil)-free custom values were covered by scenDefer; explicit panics of every
	// built-in kind
	each(func() { g.ln("panic(%s)", g.expr(t, 2)) })
	if g.has("errors") {
		each(func() { g.ln("panic(fmt.Errorf(\"wrapped: %%w\", %s{1, \"w\"}))", g.panicErrType()) })
	}
	if g.has("chan") {
		each(func() {
			g.ln("var c chan int")
			g.ln("close(c)")
		})
		each(func() {
			g.ln("c := make(chan int, 1)")
			g.ln("close(c)")
			g.ln("close(c)")
		})
		each(func() {
			g.ln("c := make(chan int, 1)")
			g.ln("close(c)")
			g.ln("c <- 1")
		})
	}
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

// scenSwitchFall exercises switch forms: fallthrough, no condition, init statements,
// duplicate-free cases with several values, break inside switch inside for.
func scenSwitchFall(g *Gen) {
	t := g.randIntType()
	v := g.declareVal("sw", t)
	n := g.between(3, 6)
	g.enter(fmt.Sprintf("for i := 0; i < %d; i++", n))
	g.ln("var trace []string")
	g.enter(fmt.Sprintf("switch x := (%s(i) + %s) %% 7; x", t.Name, v.Name))
	g.mid("case 0, 1:")
	g.ln("trace = append(trace, \"01\")")
	g.ln("fallthrough")
	g.mid("case 2:")
	g.ln("trace = append(trace, \"2\")")
	g.mid("case 3, 4, 5:")
	g.ln("trace = append(trace, \"345\")")
	g.ln("if i%%2 == 0 { break }")
	g.ln("trace = append(trace, \"after-break\")")
	g.mid("default:")
	g.ln("trace = append(trace, \"default\", fmt.Sprint(x))")
	g.leave()
	g.ln("switch {")
	g.ln("case i > %d:", g.between(0, n))
	g.ln("\ttrace = append(trace, \"big\")")
	g.ln("\tfallthrough")
	g.ln("case i == 1:")
	g.ln("\ttrace = append(trace, \"one\")")
	g.ln("default:")
	g.ln("\ttrace = append(trace, \"other\")")
	g.ln("}")
	g.ln("fmt.Println(i, trace)")
	g.leave()
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

// genTail makes the function that ends a program with an uncaught panic: the program prints
// a line and then panics with one of the kinds, whose first stderr line is compared.
func (g *Gen) genTail() string {
	name := g.topName("tail")
	kinds := []string{"string", "int", "div", "index", "nilmap", "nilptr", "custom"}
	if g.has("errors") {
		kinds = append(kinds, "error", "wrap")
	}
	k := pick(g, kinds)
	g.tail = "panic:" + k
	var b strings.Builder
	b.WriteString("func " + name + "() {\n")
	b.WriteString("\tfmt.Println(\"tail\")\n")
	switch k {
	case "string":
		b.WriteString("\tpanic(\"tail-string\")\n")
	case "int":
		b.WriteString("\tpanic(42)\n")
	case "error":
		b.WriteString("\tpanic(fmt.Errorf(\"tail-error %d\", 7))\n")
	case "div":
		b.WriteString("\tz := 0\n\tfmt.Println(10 / z)\n")
	case "index":
		b.WriteString("\ts := []int{1, 2}\n\ti := 5\n\tfmt.Println(s[i])\n")
	case "nilmap":
		b.WriteString("\tvar m map[int]int\n\tm[1] = 2\n")
	case "nilptr":
		b.WriteString("\tvar p *struct{ A int }\n\tfmt.Println(p.A)\n")
	case "custom":
		b.WriteString("\tpanic(" + g.panicErrType() + "{3, \"tail\"})\n")
	case "wrap":
		b.WriteString("\tpanic(fmt.Errorf(\"tail-wrap: %w\", " + g.panicErrType() + "{4, \"w\"}))\n")
	}
	b.WriteString("}\n")
	g.top.WriteString(b.String())
	g.top.WriteString("\n")
	return name
}
