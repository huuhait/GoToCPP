// phase: 04

// A comparison of C/C++ values is a Go bool, as in cgo: it goes into bool variables, results,
// gocpp functions and fmt, with !, && and ||. A condition takes a bool or a C++ object that
// converts to bool implicitly (the proxy of a std::vector<bool> element). &lib.global is the
// address of a C/C++ global, which lives as long as the program. A gocpp slice or map of C values
// has len, cap, append, copy and clear as any other.
package main

import (
	"fmt"

	"./cvals.h"
	"<vector>"
)

func isNil(p *cvals.node) bool { return p == nil }

func show(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func main() {
	var x C.int = 3
	var b bool = x == 3
	c := x < 2
	head := cvals.list_head()
	fmt.Println(b, c, head == nil, !(x != 3), x > 1 && head != nil, isNil(head), show(head.next != nil))
	ok := head.value == 1 || c
	if ok && !c {
		fmt.Println("ok")
	}
	var bs std.vector[bool]
	bs.push_back(true)
	bs.push_back(false)
	if bs[0] && !bs[1] {
		fmt.Println("bs")
	}
	cvals.bump_at(&cvals.the_box)
	p := &cvals.the_box
	*p += 1
	fmt.Println(int(cvals.the_box), *p == 43)

	var cs []C.int
	cs = append(cs, 1, 2)
	cs = append(cs, x)
	ds := make([]C.int, 2)
	n := copy(ds, cs)
	m := make(map[string]C.double)
	m["a"] = 1.5
	fmt.Println(len(cs), cap(cs) >= 3, n, int(ds[1]), len(m), float64(m["a"]))
	clear(ds)
	delete(m, "a")
	fmt.Println(int(ds[1]), len(m))
}

// Output:
// true false false true true false yes
// ok
// bs
// 43 true
// 3 true 2 2 1 1.5
// 0 0
