// phase: 04
package main

import "fmt"

// clamp is defined in clamp.c; no header declares it.
//
//gocpp:extern C
func clamp(x, lo, hi C.int) C.int

func main() {
	fmt.Println(clamp(15, 0, 10), clamp(-3, 0, 10))
}

// Output:
// 10 0
