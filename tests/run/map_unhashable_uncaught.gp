// phase: 05
// ExitCode: 2
// Stderr: ^panic: hash of unhashable type: \[\]int( \[recovered\])?\n\ngoroutine 1 \[running\]:\n

// A read in an empty map with a struct key that holds an unhashable interface value, not caught
// (step 5.6): Go's text has no "runtime error: " in front here.
package main

import "fmt"

type Key struct {
	id  int
	val any
}

func main() {
	m := map[Key]int{}
	fmt.Println(len(m))
	fmt.Println(m[Key{1, []int{1}}])
	fmt.Println("not reached")
}

// Output:
// 0
