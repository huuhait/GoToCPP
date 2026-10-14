// phase: 03

// print and println of a map are errors (D23): Go prints the address of its table in Go's heap
// there, 0xc000012345 (0x0 for a nil map), which gocpp can't print the same.
package main

import "fmt"

func main() {
	m := map[string]int{"a": 1}
	println(len(m), m) // ERROR "cannot println m \(map\[string\]int\): println prints a map as 0x<address>, which gocpp can't print; use fmt.Println"
	var n map[int][]int
	print(n, 1) // ERROR "cannot print n \(map\[int\]\[\]int\): print prints a map as 0x<address>"
	println(m["a"], len(n), n == nil)
	fmt.Println(len(m))
}
