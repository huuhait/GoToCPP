// phase: 02
// ExitCode: 2
// Stderr: ^panic: boom: the value was 3\n\ngoroutine 1 \[running\]:\n

// panic with a string: the output printed before it stays, the message
// goes to stderr after "panic: ", and the exit code is 2.
package main

import "fmt"

func check(v int) {
	if v > 2 {
		panic("boom: the value was 3")
	}
	fmt.Println("ok", v)
}

func main() {
	for i := range 5 {
		check(i)
	}
	fmt.Println("not reached")
}

// Output:
// ok 0
// ok 1
// ok 2
