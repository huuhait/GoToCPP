// std containers: vectors (also nested, of Go structs), iterators, maps,
// unordered maps, pairs, strings and algorithm calls.
package main

import (
	"fmt"
	"strings"

	"<algorithm>"
	"<map>"
	"<numeric>"
	"<string>"
	"<unordered_map>"
	"<utility>"
	"<vector>"
)

type Point struct{ X, Y int }

func sum(v std.vector[int]) int {
	total := 0
	for _, n := range v {
		total += int(n)
	}
	return total
}

func main() {
	v := std.vector[int]{3, 1, 2}
	v.push_back(4)
	std.sort(v.begin(), v.end())
	std.reverse(v.begin(), v.end())
	fmt.Println(sum(v), int(v.size()), v.empty())

	// iterators, also as a dotted type
	var it std.vector[int].iterator = v.begin()
	for it != v.end() {
		fmt.Println(*it)
		it++
	}
	first := std.find(v.begin(), v.end(), 2)
	if first != v.end() {
		fmt.Println("found", *first)
	}
	fmt.Println(std.accumulate(v.begin(), v.end(), 0))

	// nested templates and literals whose type is left out
	grid := std.vector[std.vector[int]]{{1, 2}, {3, 4}}
	grid.push_back(std.vector[int]{5})
	for i, row := range grid {
		for j := range row {
			fmt.Println(i, j, row[j])
		}
	}
	// The parentheses are needed (else { starts the body). gofmt and gocpp
	// fmt remove them, which breaks the code: go/printer keeps them only
	// around a literal whose type is a name, not an instance (also for Go
	// generics, G[int]{7, 8}).
	for _, x := range (std.vector[int]{7, 8}) {
		fmt.Println(x)
	}
	points := std.vector[Point]{{X: 1, Y: 2}, {3, 4}}
	points.emplace_back(Point{5, 6})
	fmt.Println(points[0].X, int(points.size()))
	builders := std.vector[strings.Builder]{}
	fmt.Println(builders.size())

	// maps
	ages := std.map[std.string, int]{}
	ages["bob"] = 31
	ages.insert(std.make_pair(std.string("amy"), 27))
	for name, age := range ages {
		fmt.Println(name, age)
	}
	counts := std.unordered_map[int, std.vector[int]]{1: {2, 3}}
	counts[2].push_back(4)
	if counts.count(1) > 0 {
		fmt.Println(int(counts[1].size()) + 1)
	}

	// pairs and multiple results from C++
	p := std.pair[int, float64]{1, 2.5}
	fmt.Println(p.first, p.second)
	lo, hi := std.minmax(3, 1)
	fmt.Println(lo, hi)
	s := std.to_string(42)
	s.append("!")
	fmt.Println(s.size(), s.c_str())
}
