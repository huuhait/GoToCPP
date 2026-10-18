// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(main\.Point\) 0x[0-9a-f]+\n\ngoroutine 1 \[running\]:\n

// A panic with a struct that is neither an error nor a Stringer prints its type and the address of
// the value (Go's printanycustomtype).
package main

type Point struct{ X, Y int }

func main() {
	panic(Point{1, 2})
}

// Output:
