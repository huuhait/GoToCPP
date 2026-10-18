// phase: 05

// Go's order of evaluation around a conversion to an interface: gc copies the value into the
// interface where the conversion is, among the calls of the statement; a value that it converts
// by value (an int, a string) is read after them (step 5.4, order.go).
package main

import "fmt"

type P struct{ a, b int }

var g P
var gi int
var gs = "s"
var gb bool

func mod() int {
	g.a++
	gi++
	gs += "!"
	gb = !gb
	return 0
}

func show(a, b any, c int) {
	fmt.Println(a.(P).a, b.(int), c)
}

func show2(a any, c int, b any) {
	fmt.Println(a.(string), c, b.(bool))
}

type Sh interface{ Area() int }
type Sq struct{ n int }

func (s Sq) Area() int { return s.n }

var gsq = Sq{1}

func modsq() int { gsq.n += 10; return 0 }

func three(a Sh, c int, b Sh) { fmt.Println(a.Area(), c, b.Area()) }

func main() {
	show(g, gi, mod())
	show(g, gi, mod())
	show2(gs, mod(), gb)
	three(gsq, modsq(), gsq)
	l := P{1, 2}
	fn := func() int { l.a = 100; return 1 }
	show(l, 5, fn())
	ps := &P{1, 1}
	f3 := func() int { ps.a = 77; return 0 }
	show2(fmt.Sprint(ps.a), f3(), true)
}

// Output:
// 1 1 0
// 2 2 0
// s!!! 0 true
// 11 0 11
// 1 5 1
// 1 0 true
