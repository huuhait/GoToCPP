// phase: 05

package main

import "fmt"

func recoverer(tag string) func() {
	return func() {
		fmt.Println(tag, "recovered:", recover())
	}
}

func plain() {
	fmt.Println("plain deferred, recover:", recover())
}

func viaVariable() {
	h := recoverer("variable")
	defer h()
	panic("a")
}

func viaReturned() {
	defer recoverer("returned")()
	panic("b")
}

func viaWrapper() {
	// The wrapper is the deferred function; recoverer's closure is not called by the defer.
	defer func() {
		recoverer("wrapped")()
	}()
	defer func() { fmt.Println("wrapper recovered:", recover()) }()
	panic("c")
}

func reassigned() {
	h := plain
	defer func() { fmt.Println("final:", recover()) }()
	h = func() { fmt.Println("reassigned:", recover()) }
	defer h()
	panic("d")
}

func main() {
	viaVariable()
	viaReturned()
	viaWrapper()
	reassigned()
}
