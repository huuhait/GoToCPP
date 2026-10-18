// phase: 05

package main

import "fmt"

// inner calls recover, but is not the deferred function: nil, the panic goes on.
func inner() any { return recover() }

func outer() {
	defer func() {
		r := inner()
		fmt.Println("inner saw:", r)
	}()
	panic("p1")
}

func main() {
	defer func() {
		fmt.Println("main recovered:", recover())
	}()
	outer()
	fmt.Println("not reached")
}
