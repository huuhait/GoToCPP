// phase: 10

// A macro that is not declared (step 10.9): the built-in macros around it still expand.

package main

import "fmt"

func main() {
	fmt.Println(#line())
	#Trace("x") // ERROR "undefined macro #Trace"
}
