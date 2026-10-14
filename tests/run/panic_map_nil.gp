// phase: 03
// ExitCode: 2
// Stderr: ^panic: assignment to entry in nil map\n

// A store into a nil map panics; reading it, its len, delete and range don't. The value is
// evaluated before the store panics.
package main

import "fmt"

func value() int {
	fmt.Println("value")
	return 1
}

func main() {
	var m map[string]int
	fmt.Println(m["a"], len(m), m == nil)
	delete(m, "a")
	for range m {
	}
	m["a"] = value()
}

// Output:
// 0 0 true
// value
