// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: hash of unhashable type main\.S

// A store with a key that can't be hashed: "runtime error: hash of unhashable type main.S".
package main

import "fmt"

type S struct{ a []int }

func main() {
	m := map[any]int{1: 1}
	fmt.Println(m[1])
	m[S{}] = 2
	fmt.Println("unreachable")
}

// Output:
// 1
