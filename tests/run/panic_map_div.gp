// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: integer divide by zero\n

// m[k] /= 0 panics with the division before it takes the entry, as gc (which makes it
// m[k] = m[k] / 0): here the map is nil, and the division panics, not the store.
package main

import "fmt"

func main() {
	m := map[string]int{"a": 6}
	d := 2
	m["a"] /= d
	fmt.Println(m["a"], len(m))
	var n map[string]int
	d = 0
	n["a"] /= d
}

// Output:
// 3 1
