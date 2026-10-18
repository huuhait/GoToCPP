// phase: 07
// ExitCode: 2
// Stderr: ^panic: interface conversion: int is not main\.Shape: missing method Area
// Stderr: (?m)^goroutine 1 \[running\]:

// A failed x.(T) in generic code that nobody recovers: the panic text and exit code.
package main

import "fmt"

type Shape interface{ Area() int }

func Must[T any](v any) T { return v.(T) }

func main() {
	fmt.Println(Must[int](1))
	fmt.Println(Must[Shape](2))
}

// Output:
// 1
