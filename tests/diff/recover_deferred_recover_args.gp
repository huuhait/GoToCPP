// phase: 05

package main

import "fmt"

func show(v any) { fmt.Println("show:", v) }

// recover() as an argument of a deferred call is evaluated at the defer statement, by the function
// that contains it: it works only when that function is itself a deferred function.
func outer() {
	defer func() {
		defer show(recover())
		fmt.Println("in deferred")
	}()
	panic("outer panic")
}

func notDeferred() {
	defer show(recover())
	fmt.Println("body")
}

func main() {
	outer()
	notDeferred()
}
