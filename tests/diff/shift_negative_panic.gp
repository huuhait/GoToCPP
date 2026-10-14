// phase: 02

// A shift by a negative int8 count panics with exit code 2, after the
// output of the shifts by 3, 0 and 100 (a count past the width gives 0
// or -1).
package main

import "fmt"

func shift(x int, n int8) int {
	fmt.Println("shift", x, n)
	return x >> n
}

func main() {
	fmt.Println(shift(1024, 3))
	fmt.Println(shift(1024, 0))
	fmt.Println(shift(-1024, 100))
	fmt.Println(shift(1024, -1))
	fmt.Println("not reached")
}
