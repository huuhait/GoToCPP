// phase: 04

// Member calls, indexing and iterators on a std::vector, with C++'s rules: v[i] has no bounds
// check, at(i) has one, and it++ on an iterator is C++'s ++it.
package main

import (
	"fmt"

	"<algorithm>"
	"<vector>"
)

func main() {
	var v std.vector[int]
	for i := 0; i < 5; i++ {
		v.push_back(i * 10)
	}
	v.push_back(7)
	fmt.Println(int(v.size()), int(v[0]), int(v[5]), int(v.at(2)), int(v.front()), int(v.back()))
	v[1] = 11 // an index of a C++ value is a place
	i := 2
	v[i] += 5
	v[i+1]++
	fmt.Println(int(v[1]), int(v[2]), int(v[3]), bool(v.empty()))
	v[0], v[1] = v[1], v[0] // Go's order: both values first
	j := 0
	j, v[j] = 4, 12 // v[j] is the old j's element
	fmt.Println(int(v[0]), int(v[1]), j)

	sum := 0
	for it := v.begin(); it != v.end(); it++ {
		sum += int(*it)
	}
	fmt.Println("sum", sum)
	std.sort(v.begin(), v.end())
	first := v.begin()
	first++
	fmt.Println(int(*first), int(*(v.end() - 1)), int(v.end()-v.begin()))
	v.pop_back()
	v.clear()
	fmt.Println(int(v.size()), bool(v.empty() == true))

	// gocpp pointers in a C++ container: their members through gp::deref.
	nodes := std.vector[*Node](2, nil)
	nodes[0] = &Node{N: 4}
	nodes[0].N++
	nodes[0].N *= 10
	fmt.Println(int(nodes[0].N), bool(nodes[1] == nil), bool(nodes[0] != nil))
	n := nodes[0]
	fmt.Println(bool(nodes[0] == n), bool(n == nodes[0]))
}

type Node struct{ N int }

// Output:
// 6 0 7 20 0 7
// 11 25 31 false
// 12 0 4
// sum 115
// 7 40 6
// 0 true
// 50 true true
// true true
