// phase: 03

// Func literals (Phase 03 step 3.11) are C++ lambdas whose bodies keep their .gp lines, one
// line per statement: gdb stops in a closure's body, prints its parameters, its locals and the
// variables it captured (a boxed one is a box: print *count), and next steps through the
// body's lines; next over a statement that makes a closure steps over the whole literal, to
// the next statement. bt names a closure's frame by its Go name (gpkg_main::main.func1);
// between it and the frame that called the func value come the frames of gp::func, which bt
// leaves out, as it leaves out those of a deferred call. A literal called at once is called
// directly. A closure in a method is named after the method: gpkg_main::box::add.func1.
package main

import "fmt"

func apply(f func(int) int, x int) int {
	return f(x)
}

type box struct{ n int }

func (b *box) add(k int) int {
	f := func() int {
		b.n += k // gdb: print k = 5; print b->n = 0; bt gpkg_main::box::add.func1 gpkg_main::box::add gpkg_main::main
		return b.n
	}
	return f()
}

func main() {
	_ = (&box{}).add(5)
	n := 10 // gdb: next 32 36
	add := func(x int) int {
		y := x + n // gdb: print x = 1; print n = 10; bt gpkg_main::main.func1 gpkg_main::apply gpkg_main::main; next 34
		return y * 2 // gdb: print y = 11
	}
	fmt.Println(apply(add, 1))
	count := 0 // gdb: next 38 42
	inc := func() {
		count++ // gdb: print *count ~ ^\(long &\) @0x[0-9a-f]+: 0$; bt gpkg_main::main.func2 gpkg_main::main; next 40
		fmt.Println("count", count) // gdb: print *count ~ ^\(long &\) @0x[0-9a-f]+: 1$
	}
	inc()
	func() {
		fmt.Println("now", n) // gdb: bt gpkg_main::main.func3 gpkg_main::main
	}()
	defer func() {
		fmt.Println("deferred", count) // gdb: print *count ~ ^\(long &\) @0x[0-9a-f]+: 1$; bt gpkg_main::main.func4 gpkg_main::main
	}()
	fmt.Println(count)
}
