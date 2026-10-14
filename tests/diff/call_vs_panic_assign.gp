// phase: 02

// x, y := a/b, f() with b == 0 calls f before the panic, as Go does (the
// review of step 2.4 found the direct path that didn't).
package main

import "fmt"

func f() int {
	fmt.Println("f called")
	return 1
}

func main() {
	a, b := 7, 0
	fmt.Println("start")
	x, y := a/b, f()
	fmt.Println("not reached", x, y)
}
