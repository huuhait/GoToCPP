// phase: 04

// Blank range targets don't read or copy discarded keys/elements, including noncopyable ones.
package main

import (
	"fmt"
	"./crange.h"
)

func main() {
	m := crange.owners_map()
	n := 0
	for range m { n++ }
	for _ = range m { n++ }
	for _, _ = range m { n++ }
	for range crange.owners_map() { n++ }
	for k := range crange.mapped_owners() { n += int(k) }
	for k, _ := range crange.mapped_owners() { n += int(k) }
	for _, v := range crange.keyed_owners() { n += int(v) }
	var v C.int
	for _, v = range crange.keyed_owners() { n += int(v) }
	fmt.Println(n, int(v), int(m.size()))
}

// Output:
// 24 7 1
