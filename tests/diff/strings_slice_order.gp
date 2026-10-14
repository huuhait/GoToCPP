// phase: 03

// Go's compiler evaluates a slice expression in its place among the calls of its
// statement: it reads the string and the bounds there, and checks the bounds there
// (a later call doesn't run when they are out of range). Its bounds that are not a
// name, a constant or len of a name are evaluated in their places too.
package main

import "fmt"

var g = "abcdef"
var n = 1

func shift(tag string) int {
	fmt.Println("shift", tag)
	g, n = "XYZ123", 3
	return 1
}

func reset() {
	g, n = "abcdef", 1
}

func main() {
	fmt.Println(g[n:], shift("a"))
	reset()
	fmt.Println(g[1:n+1], g[n:len(g)], shift("b"))
	reset()
	x := g[n:] + g[:shift("c")]
	fmt.Println(x)
	reset()
	fmt.Println(len(g[n:]), g[n:] == "bcdef", g+g[n:], g+"-", shift("d"))
	reset()
	fmt.Println(g[n+0:shift("e")+2], n)
	reset()
	fmt.Println(g[n:shift("f")+2], n)
	reset()
	fmt.Println(g[:n+1][n:], shift("g"), g[n:][:n])
	reset()
	for i, r := range g[n:] + g[:shift("h")] {
		fmt.Println(i, r)
	}
	reset()
	n = 9
	fmt.Println("before")
	fmt.Println(g[n:], shift("never"))
}
