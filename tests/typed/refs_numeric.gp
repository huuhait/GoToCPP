// phase: 12
// opaque: reject

// References and C++'s conversions. A T& result is an lvalue: it is assigned to, changed in place
// and its address taken. A C++ number meets a gocpp number by C++'s rules: no explicit conversion.
// *p, p + n and it + n are C++'s; a std::map takes a gocpp string as its key.
package main

import (
	"fmt"

	"<cmath>"
	"<cstring>"
	"<map>"
	"<string>"
	"<utility>"
	"<vector>"
)

func main() {
	v := std.vector[int]{10, 20, 30, 40}
	v.at(0) = 5
	v.back()++
	v.front() += 2
	p := &v.at(1)
	*p = 99
	it := v.begin() + 2
	*it = 77
	fmt.Println(v.at(0), v.at(1), v.at(2), v.at(3), *p, *it)

	// A reference result copied into a variable is a copy.
	c := v.at(1)
	c++
	fmt.Println(c, v.at(1))

	// Numbers: C++'s conversions, in a declaration, an assignment, an argument and a result.
	var n int = cstring.strlen("hello")
	var m int
	m = cstring.strlen("12345")
	total := cstring.strlen("abc") + len("hello")
	var cn C.int = n
	var f float64 = cmath.sqrt(16.0)
	var cut int = cmath.floor(3.9)
	fmt.Println(n, m, int(total), int(cn), f, cut)
	if cstring.strlen("abcd") > n-2 && n < cstring.strlen("hello world") {
		fmt.Println("compared")
	}

	// A std::map takes a gocpp string as its key.
	counts := std.map[std.string, int]{}
	for _, w := range []string{"a", "b", "a", "c", "a"} {
		counts[w]++
	}
	key := "b" + "!"
	counts[key] = 42
	fmt.Println(counts["a"], counts["b"], counts[key], counts.size())

	// Move semantics: the moved-from vector is empty.
	a := std.vector[int]{1, 2, 3}
	b := std.move(a)
	fmt.Println(b.size(), a.size())

	// A pair: its members, and a swap of two references.
	pr := std.make_pair(3, 4)
	pr.first += 10
	std.swap(pr.first, pr.second)
	fmt.Println(pr.first, pr.second)
}

// Output:
// 7 99 77 41 99 77
// 100 99
// 5 5 8 5 4 3
// compared
// 3 1 42 4
// 3 0
// 4 13
