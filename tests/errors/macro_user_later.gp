// phase: 10

// User macros come with step 10.9: until then a declaration or a use is an error that says so; the
// built-in macros around them still expand (Phase 10 step 10.8).

package main

import "fmt"

func main() {
	fmt.Println(#line())
	#Trace("x") // ERROR "macro #Trace is not supported yet"
}
