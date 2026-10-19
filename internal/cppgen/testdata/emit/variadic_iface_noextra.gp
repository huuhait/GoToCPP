package main

import "fmt"

// A call of a variadic function with no argument for the variadic parameter still converts its
// other arguments to their parameter types: f(x) for f(p any, q ...int) passes x as an interface
// (found by the 15.8 fuzzer: g++ said "could not convert 'x' from 'double' to 'gp::any'").

func show(p any, rest ...int) { fmt.Println(p, len(rest)) }
func all(p any, rest ...any)  { fmt.Println(p, len(rest)) }
func mid(p any, n int, rest ...string) {
	fmt.Println(p, n, len(rest))
}

type Celsius float64

var gf float64 = 3.25

func main() {
	var x float64 = 1.5
	var i int = 2
	show(x)
	show(i)
	show(gf)
	show(Celsius(7))
	show(x, 1, 2)
	all(x)
	all(i, i)
	mid(x, 1)
	mid(i, 2, "a")
	show(gf, []int{4, 5}...)
}
