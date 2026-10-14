// defer (Phase 03 step 3.13): the single-exit shape of a function with defers (the results as
// variables, a frame, the body in a try block, one exit per return), the deferred calls of the
// package's functions (gp_df.gp_push) and of builtins and fmt (a generic lambda), with their
// arguments evaluated at the defer statement in Go's order, defers in loops (last in, first
// out), a named result changed by a deferred call through a pointer, results without names,
// several results and return f(), bare returns, a return in a loop and in a switch, a boxed
// parameter, a shadowing declaration and a goto in a body with defers, defers in main and in
// init, recursion, and functions without defers, which keep their shape.
package main

import "fmt"

var trail []string

func note(s string) { trail = append(trail, s) }

func inc(p *int) int {
	*p++
	return *p
}

func show(tag string, a, b int) { fmt.Println(tag, a, b) }

func set(p *int, v int) { *p = v }

func add(p *int, v int) { *p += v }

func named() (n int) {
	defer set(&n, 42)
	n = 1
	return 7
}

func twice() (n int) {
	defer add(&n, 10)
	defer add(&n, 100)
	return 5
}

func bare() (s string, ok bool) {
	defer note("bare")
	s, ok = "x", true
	return
}

func pair(x int) (int, string) {
	defer println("pair", x)
	if x > 0 {
		return x, "pos"
	}
	return -x, "neg"
}

func swap() (a, b int) {
	defer note("swap")
	a, b = 1, 2
	return b, a
}

func forward() (int, string) {
	defer note("forward")
	return pair(-4)
}

func early(xs []int) int {
	defer note("early")
	for i, x := range xs {
		if x < 0 {
			return i
		}
	}
	return -1
}

func kind(n int) string {
	defer note("kind")
	switch {
	case n < 0:
		return "neg"
	case n == 0:
		return "zero"
	}
	return "pos"
}

func loop() {
	for i := range 3 {
		defer fmt.Println("loop", i)
	}
	fmt.Println("loop body")
}

func order() {
	z := 1
	defer show("z", z, inc(&z))
	g := []int{10, 20, 30}
	i := 0
	defer show("g", g[i], inc(&i))
	defer fmt.Println("printed", z, i)
	z, i = 100, 2
}

func builtins() {
	m := map[string]int{"a": 1, "b": 2}
	defer fmt.Println("after delete", len(m))
	defer delete(m, "a")
	s := []int{1, 2, 3}
	defer fmt.Println("after copy", s[0], s[1], s[2])
	defer copy(s, []int{7, 8})
	c := []int{4, 5}
	defer fmt.Println("after clear", c[0], c[1])
	defer clear(c)
	defer print("print ", 1, "\n")
	defer println()
}

func boxed(n int) int {
	defer add(&n, 1)
	defer fmt.Println("boxed", n)
	n *= 2
	return n
}

func shadow(v int) {
	defer fmt.Println("shadow outer", v)
	if v > 0 {
		v := v * 10
		defer fmt.Println("shadow inner", v)
	}
	w := v + 1
	defer fmt.Println("shadow w", w)
}

func jumps(n int) (r int) {
	defer note("jumps")
	i := 0
again:
	if i < n {
		i++
		r += i
		goto again
	}
	return
}

func fact(n int) (r int) {
	defer add(&r, 0)
	if n <= 1 {
		return 1
	}
	return n * fact(n-1)
}

func noDefer(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func init() {
	defer fmt.Println("init deferred")
	fmt.Println("init")
}

func main() {
	defer fmt.Println("main deferred", len(trail))
	defer note("main")
	fmt.Println(named(), twice())
	fmt.Println(bare())
	fmt.Println(pair(3))
	fmt.Println(pair(-2))
	fmt.Println(swap())
	fmt.Println(forward())
	fmt.Println(early([]int{1, -1}), early(nil))
	fmt.Println(kind(-1), kind(0), kind(1))
	loop()
	order()
	builtins()
	fmt.Println(boxed(4))
	shadow(3)
	fmt.Println(jumps(4))
	fmt.Println(fact(5))
	fmt.Println(noDefer(-3))
	fmt.Println(joined())
}

func joined() string {
	s := ""
	for _, t := range trail {
		s += t + " "
	}
	return s
}
