// phase: 05
// ExitCode: 2
// Stderr: ^panic: not found: gopher

// panic(err) with a custom error type that nothing recovers prints its Error() text (step 5.4
// with 5.10).
package main

import "fmt"

type NotFound struct{ Name string }

func (n *NotFound) Error() string { return "not found: " + n.Name }

type Shape interface{ Area() int }

func find(name string) error {
	return &NotFound{name}
}

func main() {
	defer fmt.Println("deferred runs")
	var s Shape
	if err := find("gopher"); err != nil {
		fmt.Println("about to panic", s == nil)
		panic(err)
	}
}

// Output:
// about to panic true
// deferred runs
