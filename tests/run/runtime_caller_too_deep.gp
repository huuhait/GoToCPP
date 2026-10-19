// phase: 08

// Caller with a skip beyond the stack reports ok = false and zero values.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	pc, file, line, ok := runtime.Caller(1000)
	fmt.Println(pc, file == "", line, ok)
}

// Output:
// 0 true 0 false
