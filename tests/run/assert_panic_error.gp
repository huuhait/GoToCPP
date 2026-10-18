// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: error is \*main\.E1, not \*main\.E2

// error is *main.E1, not *main.E2: an assertion on an error value (step 5.5).
package main

import "fmt"

type E1 struct{}
type E2 struct{}

func (*E1) Error() string { return "e1" }
func (*E2) Error() string { return "e2" }

func main() {
	var err error = &E1{}
	fmt.Println(err.Error())
	e2 := err.(*E2)
	fmt.Println(e2.Error())
}

// Output:
// e1
