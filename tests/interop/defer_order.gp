// phase: 04

// The object and the arguments of a deferred call are evaluated when the defer statement runs, in
// Go's order: the object of a member call (here a temporary) first, then the arguments, from left to
// right. The frame stores the object as one more argument of its call, and C++ doesn't order those,
// so g++ and clang++ used to differ (g++ evaluated the arguments first).
package main

import (
	"fmt"

	"./objects.h"
	"<vector>"
)

func g(s string) int {
	fmt.Println("g", s)
	return 2
}

func main() {
	defer objects.show2(objects.mk("a"), objects.mk("b"))
	defer objects.show2(objects.mk("c"), C.int(g("d")))
	v := std.vector[int]{}
	defer v.push_back(objects.mk("i") + C.int(g("j")))
	defer objects.mkc("k").add(objects.mk("l"))
	defer objects.mkc("m").add(objects.mk("n") + C.int(g("o")))
	fmt.Println("body")
}

// Output:
// mk a
// mk b
// mk c
// g d
// mk i
// g j
// mkc k
// mk l
// mkc m
// mk n
// g o
// body
// show2 1 2
// show2 1 1
