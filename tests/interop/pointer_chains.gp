// phase: 04
// ExitCode: 2
// Stderr: (?m)^panic: runtime error: invalid memory address or nil pointer dereference$

// Members of raw pointers to C structs, as Go's p.f: chains (p.next.next.value), comparisons with
// nil (nullptr), *p, a member that is a macro (st.when is st.when_sec), a C function pointer
// returned by a call (cvals.get_add()(2, 3)), and a nil pointer, which panics like Go's nil
// dereference instead of crashing.
package main

import (
	"fmt"

	"./cvals.h"
)

func main() {
	head := cvals.list_head()
	fmt.Println(int(head.value), int(head.next.value), int(head.next.next.value), bool(head.next.next.next == nil))
	sum := 0
	for p := cvals.list_head(); p != nil; p = p.next {
		sum += int(p.value)
	}
	fmt.Println("sum", sum)
	head.next.value = 20
	head.next.next.value *= 10
	fmt.Println(int(cvals.n2.value), int(cvals.n3.value), int((*head).value), int((*head.next).next.value))

	var none *cvals.node
	if none == nil && head != nil {
		fmt.Println("none is nil")
	}
	add := cvals.get_add()
	fmt.Println(int(cvals.get_add()(2, 3)), int(add(40, 2)), bool(add != nil))

	var st cvals.stamp
	st.when = 99
	fmt.Println(int(st.when_sec))

	fmt.Println(int(none.value))
}

// Output:
// 1 2 3 true
// sum 6
// 20 30 1 30
// none is nil
// 5 42 true
// 99
