// phase: 05

package main

import "fmt"

// A variable named recover is not the builtin, and a recover in a func literal that is called
// by the deferred function is not the deferred function's.
func f() {
	defer func() {
		recover := func() string { return "my recover" }
		fmt.Println(recover())
	}()
	defer func() {
		r := func() any { return recover() }()
		fmt.Println("literal in deferred:", r)
	}()
	defer func() {
		fmt.Println("deferred itself:", recover())
	}()
	panic("p")
}

func main() {
	f()
	fmt.Println("done")
}
