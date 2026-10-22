// phase: 05

// A package variable of a struct type with a field of an unnamed interface type, and nothing assigns
// into the field (the repro of the audit row L246).
package main

import "fmt"

type W struct{ mu interface{ Lock() } }

var w W

func main() {
	fmt.Println(w.mu == nil)
}

// Output:
// true
