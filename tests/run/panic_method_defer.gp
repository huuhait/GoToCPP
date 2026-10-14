// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: integer divide by zero\n\ngoroutine 1 \[running\]:\nmain\.Ratio\.Value\(.*\)\n

// A deferred method call stores a copy of its value receiver at the defer statement (step 3.10):
// the changes after it don't reach the call, which panics when the function returns; the calls
// deferred before it still run.
package main

import "fmt"

type Ratio struct{ a, b int }

func (r Ratio) Value() int {
	fmt.Println("value of", r.a, r.b)
	return r.a / r.b
}

func main() {
	r := Ratio{1, 0}
	defer fmt.Println("deferred first, runs last")
	defer r.Value()
	r.b = 1
	fmt.Println(r.Value())
}

// Output:
// value of 1 1
// 1
// value of 1 0
// deferred first, runs last
