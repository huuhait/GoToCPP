// phase: 02
// ExitCode: 2
// Stderr: ^panic: runtime error: negative shift amount\n

// A shift by a negative count panics; a shift by a big count gives 0.
package main

import "fmt"

func main() {
	x := 1
	n := 70
	fmt.Println(x<<n, x>>n, -8>>n)
	n = -1
	fmt.Println(x << n)
}

// Output:
// 0 0 -1
