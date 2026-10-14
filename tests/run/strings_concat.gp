// phase: 02

// String concatenation and comparison, constants and float output.
package main

import "fmt"

const greeting = "hello"

func join(a, b string) string {
	return a + ", " + b
}

func main() {
	s := join(greeting, "gocpp")
	fmt.Println(s)
	fmt.Println(s == "hello, gocpp", s < "hello", greeting+"!")
	x := 2.5
	y := x * 4
	fmt.Println(x, y, y*1.1, 1e21, 0.00001)
	fmt.Printf("%v|%f|%t\n", y, x, s != "")
}

// Output:
// hello, gocpp
// true false hello!
// 2.5 10 11 1e+21 1e-05
// 10|2.500000|true
