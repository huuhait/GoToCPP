// phase: 03

package main

import "fmt"

var trace []string

func note(s string, v int) int {
	trace = append(trace, s)
	return v
}

var gf = func(v int) int { return note("old gf", v) }

func setgf() int {
	gf = func(v int) int { return note("new gf", v) }
	return 1
}

var fs = []func(int) int{
	func(v int) int { return note("fs0", v) },
	func(v int) int { return note("fs1", v) },
}

func setfs() int {
	fs[1] = func(v int) int { return note("new fs1", v) }
	return 2
}

func mk(tag string) func(int) int {
	note("mk "+tag, 0)
	return func(v int) int { return note("made "+tag, v) }
}

func show() {
	for _, s := range trace {
		fmt.Print(s, "; ")
	}
	fmt.Println()
	trace = trace[:0]
}

func main() {
	// A variable is read after the calls of the arguments.
	gf(setgf())
	show()
	// A func value with a call in it is evaluated before the arguments.
	fs[note("index", 1)](setfs())
	show()
	fs[1] = func(v int) int { return note("fs1", v) }
	i := 1
	fs[i](setfs())
	show()
	// A call that gives the func runs before the arguments.
	mk("a")(note("arg", 3))
	fmt.Println(mk("b")(note("x", 1)) + mk("c")(note("y", 2)))
	show()
	// A closure reads its variables when it runs, after the calls before it.
	y := 1
	g := func() int { return note("g", y) }
	z := note("before", 0) + g()
	y = 5
	fmt.Println(z, g(), note("after", y))
	show()
	// defer evaluates the func value and the arguments at once.
	func() {
		h := func(v int) { fmt.Println("deferred h", v) }
		defer h(note("defer arg", 7))
		h = nil
		fmt.Println("in func")
	}()
	show()
}
