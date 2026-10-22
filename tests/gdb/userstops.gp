// phase: 05

// The stops of the user's own are not gp catch panic's: a breakpoint on a runtime function and a
// catch throw keep the runtime frame as their selected one, also with gp catch panic set. (Only
// a stop that gp catch panic made selects the Go function.) The session is userstops.gdb.
package main

import "fmt"

func direct() {
	panic("direct") // @direct
}

func unrecovered() {
	defer fmt.Println("deferred")
	panic("unrecovered") // @unrecovered
}

func main() {
	func() {
		defer func() { recover() }()
		direct()
	}()
	unrecovered()
}
