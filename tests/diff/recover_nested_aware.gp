// phase: 05

package main

import "fmt"

func guarded(name string, f func()) {
	defer func() {
		// An aware literal inside an aware function: each has its own ticket.
		defer func() { fmt.Println(name, "inner deferred sees:", recover()) }()
		fmt.Println(name, "outer deferred sees:", recover())
		if name == "again" {
			panic("second panic")
		}
	}()
	f()
}

func main() {
	guarded("plain", func() { panic("first panic") })
	guarded("again", func() { panic("first panic") })
	guarded("quiet", func() {})
	defer func() {
		defer func() { fmt.Println("main inner:", recover()) }()
		fmt.Println("main outer:", recover())
		panic("main second")
	}()
	panic("main first")
}
