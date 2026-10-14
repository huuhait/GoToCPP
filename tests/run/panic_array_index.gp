// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[3\] with length 3\n

// An index out of range of an array panics with the index and the length, also in a store.
package main

import "fmt"

func main() {
	a := [3]int{1, 2, 3}
	for i := range a {
		a[i] *= 2
		fmt.Println(a[i])
	}
	i := len(a)
	a[i] = 1
}

// Output:
// 2
// 4
// 6
