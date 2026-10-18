// phase: 07

// Generic functions of other packages as func values, passed to other generic functions, and generic
// functions that call generic functions of a package that main does not import.
package main

import (
	"fmt"

	"example.com/gvalues/lib"
)

func main() {
	inc := lib.Inc[int]
	finc := lib.Inc[float64]
	fmt.Println(inc(1), finc(1.5))
	t := lib.Traced(inc)
	fmt.Println(t(10), lib.Trace())
	h := lib.Traced(func(s string) string { return s + "!" })
	fmt.Println(h("a"), lib.Trace())
	c := lib.Compose(inc, func(i int) string { return fmt.Sprint("<", i, ">") })
	fmt.Println(c(4), lib.Traced(lib.Inc[int])(100), len(lib.Trace()))
}


// Output:
// 2 2.5
// 12 [11 12]
// a!! [11 12 a! a!!]
// <5> 102 6
