// phase: 12
// error: strtol is given the address of a char pointer

// strtol stores a pointer into the string in end: the C string of a gocpp string lives only for the
// statement.
package main

import (
	"fmt"

	"<cstdlib>"
)

func main() {
	s := "12" + fmt.Sprint(3)
	var end *C.char
	n := cstdlib.strtol(s, &end, 10)
	fmt.Println(n, end != nil)
}
