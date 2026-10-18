// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type main\.Rec\n\ngoroutine 1 \[running\]:\n

// Two interface values of the same uncomparable dynamic type (a struct with a slice field),
// compared and not caught (step 5.6).
package main

import "fmt"

type Rec struct {
	name string
	tags []string
}

func main() {
	var a, b any = Rec{name: "a"}, Rec{name: "b"}
	var c any = 1
	fmt.Println(a == c)
	fmt.Println(a == b)
	fmt.Println("not reached")
}

// Output:
// false
