// phase: 04

// Keys of gocpp maps with C++ parts: a C++ class with std::hash and == (mk.Pt), an array of C++
// strings (element by element: gp::array's gp_hash), a gocpp struct with a std.string field (its
// gp_hash and == over the fields, typedecl.go: cKeyMembers). fmt prints a map with C++ keys sorted
// as Go does: a C++ string by its compare, a class by its operator<.
package main

import (
	"fmt"

	"./map_cxx_key_class.hpp"
	"<string>"
)

type K struct {
	name std.string
	n    int
}

type Outer struct {
	k K
	b bool
}

func main() {
	pts := map[mk.Pt]int{}
	pts[mk.Pt{x: 2, y: 1}] = 21
	pts[mk.Pt{x: 1, y: 5}] = 15
	pts[mk.Pt{x: 1, y: 2}] = 12
	fmt.Println(len(pts), pts[mk.Pt{x: 1, y: 5}], pts)

	arr := map[[2]std.string]int{}
	arr[[2]std.string{"a", "b"}] = 1
	arr[[2]std.string{"a", "c"}] = 2
	fmt.Println(len(arr), arr[[2]std.string{"a", "b"}], arr[[2]std.string{"b", "a"}])

	ks := map[K]int{}
	ks[K{name: std.string("a"), n: 1}] = 5
	ks[K{name: std.string("a"), n: 2}] = 6
	fmt.Println(len(ks), ks[K{name: std.string("a"), n: 1}], ks[K{n: 1}])
	fmt.Println(ks, arr)
	os := map[Outer]bool{{k: K{name: std.string("x")}, b: true}: true}
	fmt.Println(len(os), os[Outer{k: K{name: std.string("x")}, b: true}])

	words := map[std.string]int{"b": 2, "a": 1, "c": 3}
	fmt.Println(words)
	fmt.Printf("%v %d\n", words, len(words))
}

// Output:
// 3 15 map[(1,2):12 (1,5):15 (2,1):21]
// 2 1 0
// 2 5 0
// map[{a 1}:5 {a 2}:6] map[[a b]:1 [a c]:2]
// 1 true
// map[a:1 b:2 c:3]
// map[a:1 b:2 c:3] 3
