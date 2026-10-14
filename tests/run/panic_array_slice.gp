// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[:4\] with length 3\n

// Slicing an array past its length panics with "with length N".
package main

import "fmt"

func main() {
	a := [3]int{1, 2, 3}
	s := a[1:3]
	fmt.Println(len(s), cap(s))
	j := 4
	s = a[:j]
	fmt.Println(len(s))
}

// Output:
// 2 2
