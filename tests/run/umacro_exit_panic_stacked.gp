// phase: 05

// Stacked start-style macros on a function that panics (exit criterion of Phase 10): the exit parts
// run in the opposite order of the macro lines (the first line is the outermost), each after the
// function's own defers, and a macro whose exit part recovers ends the panic for the ones above it.
// The twin is testdata/macro_twin/umacro_exit_panic_stacked.go.
package main

import "fmt"

// Trace prints when the function starts and when it ends.
macro Trace(tag string) func() {
	fmt.Println("enter", tag)
	return func() { fmt.Println("exit ", tag) }
}

// Safe stops the panic of the function in its exit part.
macro Safe() func() {
	return func() {
		if r := recover(); r != nil {
			fmt.Println("safe recovered:", r)
		}
	}
}

#Trace("outer")
#Safe()
#Trace("inner")
func guarded() int {
	defer fmt.Println("own defer of guarded")
	panic("boom")
}

#Trace("outer")
#Trace("inner")
func unguarded() int {
	defer fmt.Println("own defer of unguarded")
	panic("bang")
}

func main() {
	fmt.Println("guarded:", guarded())

	func() {
		defer func() { fmt.Println("main recovered:", recover()) }()
		unguarded()
	}()
}

// Output:
// enter outer
// enter inner
// own defer of guarded
// exit  inner
// safe recovered: boom
// exit  outer
// guarded: 0
// enter outer
// enter inner
// own defer of unguarded
// exit  inner
// exit  outer
// main recovered: bang
