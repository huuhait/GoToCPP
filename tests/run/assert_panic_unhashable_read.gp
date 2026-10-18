// phase: 05
// ExitCode: 2
// Stderr: ^panic: hash of unhashable type: \[\]int

// A read in an empty map with a key whose dynamic type can't be hashed panics with Go's text
// "hash of unhashable type: []int" (no "runtime error: ": steps 5.4/5.5, the minimal 5.6).
package main

import "fmt"

func main() {
	m := map[any]int{}
	var k any = []int{1}
	fmt.Println(len(m))
	fmt.Println(m[k])
}

// Output:
// 0
