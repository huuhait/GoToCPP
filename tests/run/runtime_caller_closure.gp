// phase: 08

// The names of closures: main.main.func1, func2 and so on, numbered in the order of the source
// (a closure that is made inside another one gets a longer name, which Go gives differently
// when it inlines the outer one, so it is not tested).
package main

import (
	"fmt"
	"runtime"
)

func name() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func main() {
	a := func() string { return name() }
	fmt.Println(a())
	b := func() string { return name() }
	fmt.Println(b())
	done := make(chan string)
	go func() { done <- name() }()
	fmt.Println(<-done)
}

// Output:
// main.main.func1
// main.main.func2
// main.main.func3
