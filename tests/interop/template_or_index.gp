// phase: 04

// [...] after a C/C++ name is a template's arguments when the kind probe says the name is a
// template (std.make_unique[T], std.get[0], std.is_same_v[A, B], a variable template), and an
// index otherwise (cvals2.table[1]). In parentheses, a C/C++ value is always indexed:
// (cvals2.arr)[2]; and (x).m is always a member with ".".
package main

import (
	"fmt"

	"./cvals2.h"
	"<limits>"
	"<memory>"
	"<string>"
	"<tuple>"
	"<type_traits>"
	"<utility>"
	"<variant>"
)

func main() {
	fmt.Println(int(cvals2.table[1]), int(cvals2.arr[0]), int((cvals2.arr)[2]), int((cvals2.table)[3]))
	cvals2.table[0] += 5
	fmt.Println(int(cvals2.table[0]), int((cvals2.arr).size()), int(cvals2.times[3](4)))
	fmt.Println(int(cvals2.size_of_v[C.int]), int(cvals2.size_of_v[C.double]))

	p := std.make_unique[std.string]("hello")
	fmt.Println(int((*p).size()), bool(p != nil), bool((*p)[0] == 'h'))
	(*p).append("!")
	q := std.make_shared[C.int](41)
	*q += 1
	fmt.Println(int((*p).size()), int(*q), int(q.use_count()))

	t := std.make_tuple(C.int(1), 2.5, std.string("three"))
	first := std.get[0](t) + 10
	fmt.Println(int(first), float64(std.get[1](t)), int(std.get[2](t).size()))
	fmt.Println(bool(std.is_same_v[C.int, C.int]), bool(std.is_same_v[C.int, C.long]))

	var w std.variant[C.int, std.string] = std.variant[C.int, std.string](std.in_place_index[1], "in place")
	fmt.Println(int(w.index()), int(std.get[1](w).size()))
	fmt.Println(int(std.numeric_limits[C.short].max()), bool(std.numeric_limits[C.uint].is_signed))
}

// Output:
// 20 7 9 40
// 15 3 12
// 4 8
// 5 true true
// 6 42 1
// 11 2.5 5
// true false
// 1 8
// 32767 false
