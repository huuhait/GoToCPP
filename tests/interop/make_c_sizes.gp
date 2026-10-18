// phase: 04

// make([]T, n) with n a C number (cgo accepts any integer type): a C int, a size_t, an enum and a
// bool-like value from C (the zlib zoo program has compressBound's unsigned long), and make of a
// map with a negative size hint, which Go ignores.
package main

import (
	"fmt"

	"./cbitfields.h"
	"<stdlib.h>"
)

func main() {
	n := cbitfields.bf_make(3, 0, 1, 0)
	a := make([]byte, n.u3)    // an unsigned bit-field
	b := make([]int, stdlib.abs(-4), cbitfields.LIGHT*2)
	c := make([]byte, cbitfields.MID)
	fmt.Println(len(a), len(b), cap(b), len(c))

	k := -1
	m := make(map[int]int, k)
	m[1] = 2
	fmt.Println(len(m))
}

// Output:
// 3 4 6 2
// 1
