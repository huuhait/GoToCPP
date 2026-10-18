// phase: 04

// Lists of C structs that gocpp builds, as long as a program makes them: built with the literal
// &T{next: head} and with n.next = head, walked with a gocpp pointer (every step of p = p.next
// converts the raw pointer back into one that owns the node), popped one by one, and freed, all
// on the default stack. Freeing a node drops the owner of the next, so a long list must not
// recurse once per node (a stack overflow, or ASan's, at some tens of thousands).
package main

import (
	"fmt"

	"./umem.hpp"
)

const n = 200000

func main() {
	var lit *umem.ucounted
	for i := 0; i < n; i++ {
		lit = &umem.ucounted{id: C.int(i), next: lit}
	}
	var st *umem.ucounted
	for i := 0; i < n; i++ {
		x := &umem.ucounted{id: C.int(i)}
		x.next = st
		st = x
	}
	fmt.Println(umem.live_counted())

	sum := 0
	for p := lit; p != nil; p = p.next {
		sum += int(p.id)
	}
	for p := st; p != nil; p = p.next {
		sum += int(p.id)
	}
	fmt.Println(sum)

	// Pop half of the list, then drop the rest of it at once.
	for i := 0; i < n/2; i++ {
		lit = lit.next
	}
	fmt.Println(int(lit.id), umem.live_counted())
	lit = nil
	fmt.Println(umem.live_counted())
	st = nil
	fmt.Println(umem.live_counted())
}

// Output:
// 400000
// 39999800000
// 99999 300000
// 200000
// 0
