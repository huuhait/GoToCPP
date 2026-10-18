// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type \[\]int

// == of two interface values with an uncomparable dynamic type panics; with different dynamic
// types it is false (steps 5.4/5.5, the minimal 5.6).
package main

import "fmt"

func main() {
	var a, b any = []int{1}, 5
	fmt.Println(a == b)
	var c, d any = []int{1}, []int{1}
	fmt.Println(c == d)
}

// Output:
// false
