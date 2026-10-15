// phase: 03

// Go's package initialization order (the spec's dependency analysis, which go/types'
// InitOrder computes): a variable is set after every variable that its initializer refers
// to, also through functions, methods, method values and func literals (even ones that are
// not called), and otherwise in declaration order. Each initializer prints when it runs.
// var a, b = f() runs f once; var _ = f() runs too. Then the init functions run in order.
package main

import "fmt"

type counter struct{ n int }

func (c counter) value() int { return trace("method", c.n+viaMethod) }

func (c *counter) bump() int { c.n++; return trace("bump", c.n) }

var (
	a         = trace("a", b+c)
	b         = trace("b", fromFunc())
	c         = trace("c", 3)
	d, e      = two()
	_         = trace("blank", d)
	f         = trace("f", lit())
	lit       = func() int { return trace("lit", g) }
	g         = trace("g", 7)
	h         = cnt.value()
	cnt       = counter{n: trace("cnt", 1)}
	viaMethod = trace("viaMethod", 100)
	mv        = cnt.value
	i         = trace("i", 1)
	p         = &cnt
	bumped    = p.bump()
	unused    = func() int { return late }
	late      = trace("late", 9)
	z         = trace("z", 0)
	sum       = trace("sum", a+d+e+f+h+bumped)
)

func fromFunc() int { return trace("fromFunc", deep()) }

func deep() int { return trace("deep", c*2) }

func two() (int, int) {
	return trace("two.1", i), trace("two.2", z)
}

var step int

func trace(name string, v int) int {
	step++
	fmt.Println(step, name, v)
	return v
}

func init() {
	fmt.Println("init 1", mv(), unused())
}

func init() {
	fmt.Println("init 2", sum, step)
}

func main() {
	fmt.Println("main", a, b, c, d, e, f, g, h, i, bumped, cnt.n, sum)
	fmt.Println(cnt, p, *p)
}
