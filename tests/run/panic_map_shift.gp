// phase: 03
// ExitCode: 2
// Stderr: ^panic: assignment to entry in nil map\n

// m[k] <<= n takes the entry before it checks the count, as gc: with a nil map and a negative
// count, the store panics, not the shift.
package main

import "fmt"

func main() {
	m := map[string]int{"a": 1}
	n := 3
	m["a"] <<= n
	fmt.Println(m["a"])
	var nm map[string]int
	n = -1
	nm["a"] <<= n
}

// Output:
// 8
