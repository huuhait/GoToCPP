// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function continued iteration after function for loop body returned false\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

func seq(yield func(int) bool) {
	yield(1)
	yield(2)
}

func main() {
	defer fmt.Println("deferred runs")
	for x := range seq {
		fmt.Println(x)
		break
	}
	fmt.Println("not reached")
}

// Output:
// 1
// deferred runs
