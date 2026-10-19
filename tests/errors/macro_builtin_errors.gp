// phase: 05

// Mistakes in the use of the built-in macros #file, #line, #func, #text and #body (Phase 10 step 10.8).

package main

import "fmt"

func main() {
	fmt.Println(#line(1))       // ERROR "#line takes no arguments"
	fmt.Println(#file(x))       // ERROR "#file takes no arguments"
	fmt.Println(#func("name"))  // ERROR "#func takes no arguments"
	fmt.Println(#text())        // ERROR "#text takes one argument"
	fmt.Println(#text(a, b))    // ERROR "#text takes one argument"
	fmt.Println(#line(xs...))   // ERROR "#line takes no arguments" "#line: cannot use \.\.\. in the arguments"
	#body()                     // ERROR "#body\(\) can only be used inside a macro declaration"
}
