// phase: 04

// A func literal that captures a local, as an argument of a call of a C++ function whose
// result goes into a new variable (x := std.count_if(..., func...)): the C++ wrapper that owns a
// pair result must not hide the captured local from clang (step 4.7), also when the call is
// hoisted by Go's order of evaluation, as an argument of fmt.Println next to another call.
package main

import (
	"fmt"

	"./crange.h"
	"<algorithm>"
	"<vector>"
)

func main() {
	limit := 1
	var v std.vector[int]
	v.push_back(1)
	v.push_back(5)
	v.push_back(7)
	n := std.count_if(v.begin(), v.end(), func(x int) bool { return x > limit })
	it := std.find_if(v.begin(), v.end(), func(x int) bool { return x > limit })
	fmt.Println(int(n), int(*it))
	var p crange.tracer
	fmt.Println(int(std.count_if(v.begin(), v.end(), func(x int) bool { return x > limit+1 })), int(p.tag(2)))
	lo, hi := std.minmax(3, 1)
	fmt.Println(int(lo), int(hi), limit)
}

// Output:
// 2 5
// 2 2
// 1 3 1
