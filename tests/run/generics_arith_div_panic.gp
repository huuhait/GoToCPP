// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: integer divide by zero
// Stderr: (?m)^goroutine 1 \[running\]:

// An uncaught division by zero in generic code: the panic text, exit code 2 and the trace names.
package main

import "fmt"

func Div[T ~int | ~int8](a, b T) T { return a / b }

func Avg[T ~int | ~int8](xs ...T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return Div(s, T(len(xs)))
}

func main() {
	fmt.Println(Avg(2, 4, 6))
	fmt.Println(Avg[int8]())
}

// Output:
// 4
