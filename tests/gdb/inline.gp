// phase: 05

// A method that the optimizer inlines keeps its Go name: with -O2 the frame of Bump is an inline
// frame of main.main (gdb's #0 and #1). The value comes from a package variable that has to be
// read at run time, so that neither compiler folds the method away and its line keeps code (a
// method of a constant local object, T.Get of frames.gp, is folded by clang++ and has no line).
// The O0 session is inline.gdb (golden), the optimized one inline.O2.gdb.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Bump(by int) int {
	c.n += by // @bump
	return c.n
}

var step = func() int { return 2 }()

func main() {
	c := &Counter{n: step}
	fmt.Println(c.Bump(step), c.Bump(step)) // @call
}
