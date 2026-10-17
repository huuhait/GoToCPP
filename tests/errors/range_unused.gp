// phase: 04

// The variables of a range over a C/C++ value follow Go's rule: one that is declared and not used
// is an error (go/types gives such a variable an invalid type and counts it as used), but a blank
// name, a variable that the body uses, and a range without variables are fine (step 4.7).
package main

import (
	"fmt"

	"<map>"
	"<vector>"
)

func main() {
	var v std.vector[int]
	for i, x := range v { // ERROR "declared and not used: i" "declared and not used: x"
	}
	for i := range v { // ERROR "declared and not used: i"
	}
	for _, x := range v { // ERROR "declared and not used: x"
	}
	for i, x := range v {
		fmt.Println(int(i), int(x))
	}
	for _, _ = range v {
	}
	for range v {
	}
	// A bare name as the key of an opaque literal may be a variable (or a C struct's field): used.
	for i := range v {
		_ = std.map[int, int]{i: 3}
	}
	for i, x := range v { // the value of a keyed element is a use too
		_ = std.map[int, int]{3: int(x)}
		_ = std.map[int, int]{int(i): 3}
	}
	var arr [2]C.int
	for i, x := range arr { // ERROR "declared and not used: i" "declared and not used: x"
	}
}
