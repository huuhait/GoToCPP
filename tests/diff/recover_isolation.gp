// phase: 05

package main

import "fmt"

// b has its own deferred recover, but runs as part of a deferred call of a panicking frame, called
// normally: its recover is not called by the panic's deferred-call machinery, so it returns nil.
func b() {
	defer func() {
		fmt.Println("b's deferred recover:", recover())
	}()
	fmt.Println("b runs")
}

func a() {
	defer func() {
		fmt.Println("a's deferred starts")
		b()
		fmt.Println("a's deferred sees the panic still alive:", recover())
	}()
	panic("a panics")
}

// The same with a deferred function that returns normally while a panic is unwinding.
func c() {
	defer func() { fmt.Println("c's outer deferred:", recover()) }()
	defer func() {
		defer func() { fmt.Println("nested deferred in a deferred:", recover()) }()
	}()
	panic("c panics")
}

func main() {
	a()
	c()
}
