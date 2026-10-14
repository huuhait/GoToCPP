// Go allows a function named like a free function of the runtime (gp::len,
// gp::panic, gp::print). A call with an argument of a C++ class type
// (gp::string) is qualified, so that C++'s argument-dependent lookup doesn't
// find the runtime's function too; a call with only numbers stays plain.
package main

import "fmt"

func len(s string) int {
	return 3
}

func panic(s string) string {
	return "no panic: " + s
}

func print(a, b string) string {
	return a + b
}

func div(a int, s string) int {
	return a + len(s)
}

func shl(x int, b bool) int {
	if0 := x
	return if0 + 1
}

func main() {
	s := "abc"
	fmt.Println(len(s), panic(s), print(s, "!"), div(1, s), shl(2, true))
	fmt.Println(len("literal"), panic("x"))
}
