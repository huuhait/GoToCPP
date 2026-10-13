// phase: 03
// ExitCode: 2
// Stderr: ^panic: boom
// Stderr: (?m)^goroutine

package main

import "fmt"

func main() {
	fmt.Println("a")
	fmt.Println()
	fmt.Println("\tb  c")
	panic("boom")
}

// Output:
// a
//
// 	b  c


