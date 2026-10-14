// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: cannot convert slice with length 2 to array or pointer to array with length 3\n

// Converting a slice that is too short to an array panics with Go's message.
package main

import "fmt"

func main() {
	s := []int{1, 2, 3}
	a := [3]int(s)
	fmt.Println(a[2])
	s = s[:2]
	p := (*[2]int)(s)
	fmt.Println(p[1])
	b := [3]int(s)
	fmt.Println(b[0])
}

// Output:
// 3
// 2
