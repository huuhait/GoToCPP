// phase: 09

// Macros together with the must operator `!` (step 10.9): a macro on top of a must function and of
// main!, a must call inside a macro (a macro has no error result, so the error panics there), a macro
// inside a must function. The twin (testdata/macro_twin/umacro_must.go) writes both rewrites by hand.

package main

import (
	"fmt"
	"strconv"
)

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

macro Parse(s string) int {
	n := strconv.Atoi!(s)
	return n
}

#Trace()
func load!(s string) int {
	n := strconv.Atoi!(s)
	return n + #Parse("1")
}

#Trace()
func main!() {
	fmt.Println(load!("41"))
	func() {
		defer func() { fmt.Println("recovered:", recover() != nil) }()
		_, err := load("x")
		fmt.Println(err != nil)
		fmt.Println(#Parse("nope"))
	}()
	fmt.Println(#Parse("7"))
}

// Output:
// enter main.main
// enter main.load
// exit  main.load
// 42
// enter main.load
// exit  main.load
// true
// recovered: true
// 7
// exit  main.main
