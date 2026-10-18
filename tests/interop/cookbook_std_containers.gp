// phase: 04

package main

import (
	"fmt"

	"<algorithm>"
	"<map>"
	"<string>"
	"<vector>"
)

func main() {
	// std.vector[int] is std::vector<int64_t>: gocpp's int is 64 bits.
	v := std.vector[int]{5, 2, 9}
	v.push_back(1)
	std.sort(v.begin(), v.end())
	fmt.Println(int(v.size()), int(v[0]), int(v.back()))

	// std.string is explicit: a Go string converts to it with std.string(s).
	s := std.string("hello")
	s += ", world"
	fmt.Println(string(s), int(s.size()), int(s.find("world")))

	// A C++ map keeps C++ rules: m[k] inserts, and the values come back as C++ values.
	ages := std.map[std.string, int]{}
	ages["bob"] = 31
	ages["alice"] = 29
	ages["bob"]++
	fmt.Println(int(ages.size()), int(ages["bob"]), int(ages.count("carol")))

	// An untyped constant is a plain C literal (an int): templates that deduce one type from two
	// arguments need a typed one.
	n := 3
	fmt.Println(int(std.max(n, int(10))))
}

// Output:
// 4 1 9
// hello, world 12 7
// 2 32 0
// 10
