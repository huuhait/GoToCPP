// The evaluation order (order.go): calls with side effects in every context, next
// to reads of the package variables that they change, as Go's compiler orders them
// (calls first, from left to right; then the rest of the expression).
package main

import "fmt"

var x = 1
var n = 0
var s = "abc"

// Package variables: their initializers are ordered too (in gp_var_init).
var first = g(x, inc())
var p1, p2 = next(), n

// next returns 1, 2, 3, ...
func next() int {
	n++
	return n
}

// inc increments x and returns its new value.
func inc() int {
	x++
	return x
}

// count returns a shift count, 64.
func count() uint {
	n++
	return 64
}

// grow makes s longer.
func grow() int {
	s += "z"
	return len(s)
}

func g(a, b int) int {
	fmt.Println("g", a, b)
	return a*10 + b
}

func t(name string, v bool) bool {
	fmt.Print(name, " ")
	return v
}

func pair() (int, int) {
	return next(), next()
}

func both(a, b int) int {
	return a*100 + b
}

func ret() (int, int) {
	return x, inc() // x is read after inc
}

func ret2() (int, int) {
	return next(), next() // a braced list: in order without temporaries
}

func ret3() int {
	return g(x, inc())
}

func main() {
	// The audit's cases: g++ and clang++ differ on both.
	fmt.Println(next(), next())
	g(x, inc())

	// Reads of package variables next to calls that change them; locals need nothing.
	local := 5
	fmt.Println(local, inc(), x, local+x)
	fmt.Println(x+inc(), inc()+x)
	y := x * inc()
	fmt.Println(y)
	println(x, inc(), n, next())
	fmt.Println(len(s), grow(), s) // len is evaluated in its place among the calls
	z := 1
	fmt.Println(1000>>(1<<count()-2)+7/z, next()) // an untyped count is a uint, also in a temporary
	// fmt's ...any copies a byte or a bool that is not a variable in its place too.
	fmt.Println(x == 2, inc(), x == 2, s[3], grow(), uint8(x))
	fmt.Printf("%v %v %v\n", x == 3, inc(), first+p1+p2)

	// Nested calls and multiple results.
	fmt.Println(g(next(), g(next(), next())))
	fmt.Println(both(pair()))
	fmt.Println(pair())
	fmt.Println(ret())
	fmt.Println(ret2())
	fmt.Println(ret3())

	// && and ||: the right side runs only if it must, its temporaries too.
	fmt.Println(t("a", true) && t("b", false), t("c", false) || t("d", true))
	fmt.Println(x > 0 && x < 6, inc()) // the && is evaluated before inc, like a call
	if t("e", false) && g(x, inc()) > 0 {
		fmt.Println("never")
	}
	if t("f", true) && g(x, inc()) > 0 {
		fmt.Println("then")
	}
	ok := t("g", false) || g(x, inc()) > 0 || t("h", true)
	fmt.Println(ok)

	// Assignments and declarations.
	a, b := x, inc()
	fmt.Println(a, b)
	a, b = x, inc()
	fmt.Println(a, b)
	a, b = b, inc()+a
	fmt.Println(a, b)
	var p, q = next(), n
	fmt.Println(p, q)
	x += inc()
	fmt.Println(x)
	a = x - inc()
	fmt.Println(a)
	_ = g(x, inc())

	// Conditions.
	if x == inc() {
		fmt.Println("never")
	} else if next() > 100 {
		fmt.Println("never")
	} else if g(x, inc()) > 0 {
		fmt.Println("else if", x)
	}
	if v := next(); v > 100 {
		fmt.Println("never")
	} else if w := g(x, inc()); w < 0 {
		fmt.Println("never")
	} else if g(w, next()) > 0 {
		fmt.Println("else if with init", v, w)
	}

	// Loop conditions run before every iteration, also after continue.
	x = 0
	for x < g(x, inc()) && x < 4 {
		if x == 2 {
			continue
		}
		fmt.Println("for cond", x)
	}
	x = 0
	for i := 0; i < x+inc() && i < 3; i++ {
		fmt.Println("3-clause", i, x)
	}
	x = 0
	for i := 0; i < 3; i = i + x - inc() + 2 { // the post needs a temporary: it goes to the end
		if i == 1 {
			continue
		}
		fmt.Println("post", i, x)
	}
	x = 0
outer:
	for i, j := x, inc(); i < 3; i++ { // the init needs a temporary: before the loop
		for k := 0; k < x+inc(); k++ {
			if k == 1 {
				continue outer
			}
			fmt.Println("init", i, j, k)
		}
	}
	x = 1
	for i := range g(x, inc()) / 10 { // evaluated once
		fmt.Println("range", i)
	}

	// switch: the tag once, the cases lazily, in order.
	x = 1
	switch g(x, inc()) {
	case next():
		fmt.Println("never")
	case g(x, inc()), next():
		fmt.Println("case", x, n)
	case g(x, inc()):
		fmt.Println("never")
	}
	switch x {
	case 0, g(x, inc()), inc():
		fmt.Println("case list", x)
	default:
		fmt.Println("default", x)
	}
	switch {
	case t("i", false):
	case t("j", false), g(x, inc()) > 0:
		fmt.Println("tagless", x)
	}
	fmt.Println(x, n, s)
}
