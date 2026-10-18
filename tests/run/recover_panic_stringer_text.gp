// phase: 05
// ExitCode: 2
// Stderr: ^panic: stringy 2\n\ngoroutine 1 \[running\]:\n

// A panic with a fmt.Stringer prints its String() text.
package main

import "fmt"

type Stringy struct{ N int }

func (s Stringy) String() string { return fmt.Sprint("stringy ", s.N) }

func main() {
	panic(Stringy{2})
}

// Output:
