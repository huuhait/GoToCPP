// phase: 08

// Where a macro can be used (step 10.9): in package-level initializers, in closures, goroutines,
// loop headers, switch cases, composite literals, returns and method values; with the names
// macro and lazy as ordinary identifiers outside macro declarations (divergence D18). The twin (testdata/macro_twin/umacro_contexts.go) is the
// same program with the macros written out.

package main

import (
	"fmt"
	"sync"
)

// tag labels a value with the place of the use.
macro tag[T any](v T) string {
	return fmt.Sprintf("%v@%d", v, #line())
}

// show prints and returns v.
macro show(v int) int {
	fmt.Println("show", v, #func())
	return v
}

// A package-level initializer: the place is the package's init.
var first = #show(1)

var table = map[string]int{
	"a": #show(10),
	"b": #show(20),
}

type Box struct {
	Label string
	N     int
}

func (b Box) Describe() string { return #tag(b.N) }

func noUse() int {
	show := 5 // a macro of the package has this name: fine, as the function has no use of it
	return show
}

func label(n int) string {
	switch n {
	case #show(1):
		return "one"
	case #show(2) + 1:
		return "three"
	}
	return "other"
}

func collect() (out []string) {
	for i := 0; i < #show(2); i++ {
		out = append(out, #tag(i))
	}
	return
}

func main() {
	// macro and lazy are ordinary names outside a macro declaration (D18), and a variable named
	// like a macro in a function without a use of it is no problem.
	macro, lazy := 1, 2
	fmt.Println(macro + lazy)

	fmt.Println(first, table["a"]+table["b"], noUse())

	b := Box{Label: "box", N: #show(5)}
	fmt.Println(b, b.Describe())
	desc := b.Describe
	fmt.Println(desc())

	fmt.Println(label(1), label(3), label(7))
	fmt.Println(collect())

	// In a closure and in goroutines.
	f := func() int { return #show(6) }
	fmt.Println(f())
	var wg sync.WaitGroup
	res := make([]int, 3)
	for i := range 3 {
		wg.Go(func() { res[i] = #show(0) + i })
	}
	wg.Wait()
	fmt.Println(res)

	// As an argument, an operand, an index, a return value of a closure and a send.
	ch := make(chan int, 1)
	ch <- #show(8)
	fmt.Println(<-ch, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}[#show(3)], #show(1)+#show(2)*#show(3))
	if v := #show(4); v > 3 {
		fmt.Println("v =", v)
	}
	goto end
end:
	fmt.Println(#tag("end"))
}

// Output:
// show 1 main.init
// show 10 main.init
// show 20 main.init
// 3
// 1 30 5
// show 5 main.main
// {box 5} 5@39
// 5@39
// show 1 main.label
// show 1 main.label
// show 2 main.label
// show 1 main.label
// show 2 main.label
// one three other
// show 2 main.collect
// show 2 main.collect
// show 2 main.collect
// [0@58 1@58]
// show 6 main.main.func1
// 6
// show 0 main.main.func2
// show 0 main.main.func2
// show 0 main.main.func2
// [0 1 2]
// show 8 main.main
// show 3 main.main
// show 1 main.main
// show 2 main.main
// show 3 main.main
// 8 4 7
// show 4 main.main
// v = 4
// end@99
