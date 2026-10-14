// A package of two files: prototypes make the order of the functions not
// matter, and the package variables of both files are initialized in Go's order.
package main

import "fmt"

var first = second + 1

func main() {
	fmt.Println(first, second, fromB())
}

func fromA() int {
	return 1
}
