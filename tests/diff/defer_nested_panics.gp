// phase: 03

// Panics in deferred calls: a deferred call that panics on a normal return, while other
// deferred calls remain (they run); a deferred function that has defers of its own and
// panics; a panic in a deferred call while a panic runs; recursion where each level's
// deferred call prints. The exit code is 2 and stdout shows which calls ran, in Go's order.
package main

import "fmt"

func say(s string) { fmt.Println(s) }

func boom(s string) {
	fmt.Println("boom", s)
	panic(s)
}

func helper() {
	defer say("helper's defer")
	defer boom("in helper's defer")
	fmt.Println("helper body")
}

func level(n int) {
	defer fmt.Println("level", n, "deferred")
	if n == 0 {
		defer helper()
		return
	}
	level(n - 1)
}

func main() {
	defer say("main's last defer")
	defer boom("from main's defer")
	level(2)
	fmt.Println("not reached")
}
