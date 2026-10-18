// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: hash of unhashable type map\[string\]int\n\ngoroutine 1 \[running\]:\n

// An assignment with an array key that holds an unhashable interface value (step 5.6).
package main

import "fmt"

func main() {
	m := map[[2]any]int{}
	m[[2]any{1, "a"}] = 1
	fmt.Println(len(m))
	m[[2]any{1, map[string]int{}}] = 2
	fmt.Println("not reached")
}

// Output:
// 1
