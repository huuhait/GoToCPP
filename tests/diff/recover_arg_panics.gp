// phase: 05

package main

import "fmt"

func show(tag string, n int) { fmt.Println("show", tag, n) }

// A panic while a deferred call's arguments are evaluated: the call is never registered, the
// calls before it still run, and a deferred recover sees the panic.
func f(d int) {
	defer func() { fmt.Println("recovered:", recover()) }()
	defer show("first", 1)
	defer show("second", 10/d)
	defer show("third", 3)
	fmt.Println("body")
}

func g() {
	var m map[string]int
	defer func() { fmt.Println("g recovered:", recover()) }()
	defer show("before", 1)
	defer func(v int) { fmt.Println("never registered", v) }(m["a"] / len(m))
	fmt.Println("g body")
}

func main() {
	f(2)
	f(0)
	g()
}
