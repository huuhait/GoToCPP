// phase: 03

// A panic while the arguments of a deferred call are evaluated (an index out of range, after
// the calls of the statement ran) happens at the defer statement: that call is never deferred,
// and the deferred calls before it run.
package main

import "fmt"

func f(tag string) int {
	fmt.Println("f", tag)
	return 7
}

func show(a, b int) { fmt.Println("show", a, b) }

func work(s []int, i int) {
	defer fmt.Println("work's first defer")
	defer show(s[i], f("args"))
	fmt.Println("not reached")
}

func main() {
	defer fmt.Println("main's defer")
	work([]int{1}, 3)
}
