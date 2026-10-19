// phase: 03

// make(map[K]V, n) with a hint whose table would overflow or pass the allocator's limit: Go
// ignores the hint (makemap checks MulUintptr), so the map is empty and works; gocpp did a
// fatal "out of memory".
package main

import "fmt"

type Huge [1 << 44]byte
type Huger [1 << 46]byte

func main() {
	a := make(map[int]Huge, 1<<22)
	b := make(map[int]Huger, 1<<10)
	c := make(map[string]int, 1<<40)
	c["x"] = 1
	fmt.Println(len(a), len(b), len(c), c["x"])
}

// Output:
// 0 0 1 1
