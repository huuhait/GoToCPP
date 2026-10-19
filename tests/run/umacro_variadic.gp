// phase: 05

// A variadic macro (step 10.13): its hidden arguments (the place of the use and the text of #text(xs)) come before the
// variadic parameter, and a call with no, one or several arguments, or with a slice and ..., gets them too. The twin
// (testdata/macro_twin/umacro_variadic.go) is the macros written out as functions.

package main

import "fmt"

macro Sum(xs ...int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	fmt.Printf("sum %s %d %q %d %d\n", #func(), #line(), #text(xs), len(xs), total)
	return total
}

macro Join(sep string, parts ...string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	fmt.Printf("join %s %d %q\n", #file(), #line(), #text(parts))
	return out
}

func main() {
	fmt.Println(#Sum())
	fmt.Println(#Sum(1))
	fmt.Println(#Sum(1, 2, 3))
	xs := []int{4, 5}
	fmt.Println(#Sum(xs...))
	fmt.Println(#Join("-", "a", "b"))
	fmt.Println(#Join("+"))
}

// Output:
// sum main.main 33 "" 0 0
// 0
// sum main.main 34 "1" 1 1
// 1
// sum main.main 35 "1, 2, 3" 3 6
// 6
// sum main.main 37 "xs" 2 9
// 9
// join umacro_variadic.gp 38 "\"a\", \"b\""
// a-b
// join umacro_variadic.gp 39 ""
//
