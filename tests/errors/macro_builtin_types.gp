// phase: 10

// The built-in macros are constants of Go (Phase 10 step 10.8): #file(), #func() and #text(x) are
// untyped string constants, #line() is an untyped integer constant, #embed("f") is an untyped string
// constant, so go/types checks how they are used.

package main

import "fmt"

func main() {
	var n int = #file()                     // ERROR "cannot use .*untyped string constant.* as int value"
	var s string = #line()                  // ERROR "cannot use .*untyped int constant.* as string value"
	var t string = #func() + 1              // ERROR "mismatched types untyped string and untyped int"
	var u int = #text(x)                    // ERROR "cannot use .*untyped string constant.* as int value"
	var v int = #embed("embeddata/x.txt")   // ERROR "cannot use .*untyped string constant.* as int value"
	var w byte = #line() + 300              // ERROR "cannot use .*untyped int constant 317.* as byte value.*overflows"
	fmt.Println(n, s, t, u, v, w)
}
