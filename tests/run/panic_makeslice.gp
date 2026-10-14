// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: makeslice: len out of range\n

// make with a negative length panics.
package main

import "fmt"

func main() {
	n := 3
	fmt.Println(len(make([]int, n)))
	n -= 4
	fmt.Println(len(make([]int, n)))
}

// Output:
// 3
