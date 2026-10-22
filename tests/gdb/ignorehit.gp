// phase: 05

// A hit of gp catch panic's own stop that an ignore count resumes leaves nothing behind: the
// breakpoints of the user's that stop afterwards keep their own frame. The session is
// ignorehit.gdb.
package main

import "fmt"

func withDefer() {
	defer func() { recover() }()
	panic("with defer") // @withdefer
}

func direct() {
	panic("direct") // @direct
}

func unrecovered() {
	defer fmt.Println("deferred")
	panic("unrecovered") // @unrecovered
}

func main() {
	withDefer()
	func() {
		defer func() { recover() }()
		direct()
	}()
	unrecovered()
}
