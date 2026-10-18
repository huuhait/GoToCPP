// phase: 05

// The deferred calls of every frame between the throw and the recover run, innermost first, and
// the function that recovers returns normally, with the named results its defers set.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func level3() {
	defer fmt.Println("level3 deferred")
	cxxvals.throw_runtime(3)
	fmt.Println("level3 not reached")
}

func level2() {
	defer fmt.Println("level2 deferred")
	level3()
	fmt.Println("level2 not reached")
}

func level1() (result int, note string) {
	defer func() {
		if r := recover(); r != nil {
			result = -1
			note = fmt.Sprint(r)
		}
	}()
	defer fmt.Println("level1 deferred")
	level2()
	return 1, "normal"
}

func main() {
	defer fmt.Println("main deferred")
	fmt.Println(level1())
	fmt.Println("main continues")
}

// Output:
// level3 deferred
// level2 deferred
// level1 deferred
// -1 C++ exception std::runtime_error: bad thing 3
// main continues
// main deferred
