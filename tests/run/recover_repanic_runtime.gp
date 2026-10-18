// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[3\] with length 3 \[recovered, repanicked\]\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r)
		panic(r)
	}()
	a := []int{1, 2, 3}
	i := 3
	fmt.Println(a[i])
}

// Output:
// recovered: runtime error: index out of range [3] with length 3
