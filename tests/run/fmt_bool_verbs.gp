// phase: 05

// Go's %t and %v of bool with width and flags; the error forms for other verbs.
package main

import "fmt"

type Flag bool

func main() {
	fmt.Printf("[%t]\n", true)
	fmt.Printf("[%5t]\n", true)
	fmt.Printf("[%-5t]\n", true)
	fmt.Printf("[%05t]\n", true)
	fmt.Printf("[%+t]\n", true)
	fmt.Printf("[%#t]\n", true)
	fmt.Printf("[%.1t]\n", true)
	fmt.Printf("[%8.3t]\n", true)
	fmt.Printf("[%t]\n", false)
	fmt.Printf("[%5t]\n", false)
	fmt.Printf("[%-5t]\n", false)
	fmt.Printf("[%05t]\n", false)
	fmt.Printf("[%+t]\n", false)
	fmt.Printf("[%#t]\n", false)
	fmt.Printf("[%.1t]\n", false)
	fmt.Printf("[%8.3t]\n", false)
	fmt.Printf("[%v]\n", true)
	fmt.Printf("[%5v]\n", true)
	fmt.Printf("[%-6v]\n", true)
	fmt.Printf("[%v]\n", false)
	fmt.Printf("[%5v]\n", false)
	fmt.Printf("[%-6v]\n", false)
	fmt.Printf("%d %s %x %c %q\n", true, false, true, true, false)
	fmt.Printf("%v %t %d\n", Flag(true), Flag(false), Flag(true))
	fmt.Printf("%v %v %t\n", []bool{true, false}, map[bool]int{true: 1, false: 0}, [2]bool{})
	fmt.Printf("%#v %#v %T\n", true, Flag(false), Flag(true))
}

// Output:
// [true]
// [ true]
// [true ]
// [0true]
// [true]
// [true]
// [true]
// [    true]
// [false]
// [false]
// [false]
// [false]
// [false]
// [false]
// [false]
// [   false]
// [true]
// [ true]
// [true  ]
// [false]
// [false]
// [false ]
// %!d(bool=true) %!s(bool=false) %!x(bool=true) %!c(bool=true) %!q(bool=false)
// true false %!d(main.Flag=true)
// [true false] map[false:0 true:1] [false false]
// true false main.Flag
