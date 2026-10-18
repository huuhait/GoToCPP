// phase: 05

package main

import "fmt"

func pair() (int, string) { return 7, "seven" }

func report(n int, s string) { fmt.Println("report", n, s) }

func join(prefix string, parts ...string) {
	fmt.Println(prefix, len(parts), parts)
}

func f() {
	defer report(pair())
	defer join("none")
	defer join("three", "a", "b", "c")
	xs := []string{"x", "y"}
	defer join("slice", xs...)
	xs[0] = "changed"
	defer fmt.Println(pair())
	panic("done")
}

func main() {
	defer func() { fmt.Println("main recovered:", recover()) }()
	f()
}
