// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[3\] with length 3\n\ngoroutine 1 \[running\]:\n

// An index out of range of a named slice type panics with Go's message (step 3.9), also with an
// index of a named integer type.
package main

import "fmt"

type Slot int
type Slots []string

func main() {
	s := Slots{"a", "b", "c"}
	for i := Slot(0); i < 4; i++ {
		fmt.Println(i, s[i])
	}
}

// Output:
// 0 a
// 1 b
// 2 c
