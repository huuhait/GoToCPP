// phase: 03

// init is not an ordinary name at package level: it must be a func without parameters and
// results, and nothing can call it or refer to it (go/types' errors, as in Go).
package main

var init = 1 // ERROR "cannot declare init - must be func"

func init(x int) {} // ERROR "func init must have no arguments and no return values"

func init() int { return 0 } // ERROR "func init must have no arguments and no return values"

func init() {}

func main() {
	init()    // ERROR "undefined: init"
	f := init // ERROR "undefined: init"
	_ = f
}
