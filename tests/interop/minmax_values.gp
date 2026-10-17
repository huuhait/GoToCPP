// phase: 04

// Several values from one C++ value, a pair or a tuple (std.minmax returns a pair of
// references): gp::values copies them, so they don't dangle (an ASan run checks it), also when
// the arguments are temporaries. With :=, =, _ and variables that exist. One variable from a
// C/C++ call goes through gp::values too (step 4.7): p := std.minmax(3, 1), var q = ..., and a
// temporary of the order of evaluation, which is a copy as Go's results are.
package main

import (
	"fmt"

	"./crange.h"
	"<algorithm>"
	"<string>"
	"<tuple>"
	"<utility>"
)

func next(n *C.int) C.int {
	*n++
	return *n
}

func main() {
	lo, hi := std.minmax(3, 1)
	fmt.Println(int(lo), int(hi))
	x, y := C.int(10), C.int(-4)
	a, b := std.minmax(x, y)
	x = 0 // a and b are copies, not references to x and y
	fmt.Println(int(a), int(b), int(x))
	lo, hi = std.minmax(std.string("pear").size(), std.string("fig").size())
	fmt.Println(int(lo), int(hi))
	_, top := std.minmax(C.int(7), C.int(9))
	n, s, d := std.make_tuple(C.int(1), std.string("two"), 3.5)
	lo, z := std.minmax(C.long(5), C.long(2))
	fmt.Println(int(top), int(n), int(s.size()), float64(d), int(lo), int(z))

	p := std.minmax(3, 1)
	var q = std.minmax(std.string("a long pear, on the heap"), std.string("a fig"))
	t := std.tie(x, y)
	x = 5 // t is a copy, not references to x and y
	fmt.Println(int(p.first), int(p.second), int(q.first.size()), int(q.second.size()), int(std.get[0](t)))
	p = std.minmax(8, 6)
	fmt.Println(int(p.first), int(p.second))
	// The pair goes into a temporary before next runs: a copy of the old values.
	k := C.int(1)
	fmt.Println(int(crange.first_of(std.minmax(k, C.int(4)), next(&k))))
	fmt.Println(int(crange.first_of(std.minmax(C.int(6), C.int(7)), next(&k))))
	heap := std.minmax(std.string(80, 'b'), std.string(90, 'a'))
	fmt.Println(int(heap.first.size()), int(heap.second.size()), int(heap.first[0]))
	// Ordinary prvalues preserve guaranteed elision, even with deleted copy and move.
	imm := crange.make_immovable()
	var imm2 = crange.make_immovable()
	mu := crange.fresh_mutex()
	mu.lock()
	mu.unlock()
	fmt.Println(int(imm.n), int(imm2.n))
	r := crange.fresh_result()
	var r2 = crange.fresh_result()
	r3 := crange.result_ref() // one copy of the referenced ordinary object
	fmt.Println(int(r.n), int(r2.n), int(r3.n), int(crange.result_copies), int(crange.result_moves))
	// Also cover an ordinary result hoisted by the evaluation order.
	fmt.Println(int(crange.read_result(crange.fresh_result(), next(&k))))
	fmt.Println(int(crange.result_copies), int(crange.result_moves))
	k = 1
	fmt.Println(int(next(&k)), int(k))
	k = 1
	fmt.Println(int(crange.increment_ref(k)), int(k))
}

// Output:
// 1 3
// -4 10 0
// 3 4
// 9 1 3 3.5 2 5
// 1 3 5 24 0
// 6 8
// 3
// 9
// 90 80 97
// 7 7
// 7 7 7 1 0
// 11
// 1 0
// 2 2
// 2 2
