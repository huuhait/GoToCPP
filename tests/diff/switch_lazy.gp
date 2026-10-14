// phase: 02

// Case expressions are evaluated lazily, top to bottom and left to right,
// only until one matches; the tag is evaluated once, before the cases.
package main

import "fmt"

func val(name string, v int) int {
	fmt.Print(name, " ")
	return v
}

func yes(name string, b bool) bool {
	fmt.Print(name, " ")
	return b
}

func main() {
	for i := range 4 {
		switch val("tag", i) {
		case val("a", 0), val("b", 1):
			fmt.Println("-> a or b")
		case val("c", 2):
			fmt.Println("-> c")
		default:
			fmt.Println("-> default")
		case val("d", 9), val("e", 3):
			fmt.Println("-> d or e")
		}
	}
	for i := range 3 {
		switch {
		case yes("x", i == 0):
			fmt.Println("-> x")
		case yes("y", i == 1), yes("z", i == 2):
			fmt.Println("-> y or z")
			fallthrough
		case yes("never", false):
			fmt.Println("-> fell through, case not evaluated")
		}
	}
	switch x := val("init", 5); val("tag", x*2) {
	case val("first", 10):
		fmt.Println("-> first")
	case val("second", 10):
		fmt.Println("-> second")
	}
}
