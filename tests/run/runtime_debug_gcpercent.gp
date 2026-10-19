// phase: 08

// debug.SetGCPercent returns the previous value (100 at first) and remembers the new one.
package main

import (
	"fmt"
	"runtime/debug"
)

func main() {
	fmt.Println(debug.SetGCPercent(50))
	fmt.Println(debug.SetGCPercent(-1))
	fmt.Println(debug.SetGCPercent(100))
	fmt.Println(debug.SetGCPercent(100))
}

// Output:
// 100
// 50
// -1
// 100
