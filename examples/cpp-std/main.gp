// phase: 04

// Cpp-std uses the C++ standard library directly: std::vector, std::map,
// std::string, <algorithm> and std::filesystem. Importing a header makes its
// namespace usable like a package (std.vector, std.sort), with no glue code.
// C++ values keep C++ rules: reading a missing key of a std::map inserts it,
// and a std::vector has no bounds checks.
package main

import (
	"fmt"

	"<algorithm>"
	"<filesystem>"
	"<map>"
	"<string>"
	"<vector>"

	fs "::std::filesystem"
)

func main() {
	// A std::vector<int64_t>: gocpp's int is 64 bits.
	nums := std.vector[int]{5, 3, 8, 1}
	nums.push_back(4)
	std.sort(nums.begin(), nums.end())
	var sorted []int
	for _, n := range nums {
		sorted = append(sorted, n)
	}
	fmt.Println("sorted:", sorted, "size:", int(nums.size()))
	fmt.Println("front:", nums.front(), "back:", nums.back(), "at(1):", nums.at(1))
	pos := std.find(nums.begin(), nums.end(), 5) - nums.begin()
	fmt.Println("index of 5:", int(pos), "max:", std.max(nums.front(), nums.back()))
	lo, hi := std.minmax(3, 1)
	fmt.Println("minmax:", lo, hi)

	// A std::map keeps its keys sorted.
	ages := std.map[std.string, int]{}
	ages["bob"] = 31
	ages["alice"] = 29
	ages["carol"] = 35
	ages["bob"]++
	for name, age := range ages {
		fmt.Println(name, age)
	}
	dave := ages["dave"] // inserts "dave", as operator[] does in C++
	fmt.Println("dave:", dave, "size:", int(ages.size()), "has eve:", ages.count("eve") == 1)

	// A std::string is a C++ value; string(x) and std.string(x) convert.
	s := std.string("hello")
	s.append(", world")
	s += "!"
	fmt.Println(s, int(s.size()))
	if at := s.find("world"); at != std.string.npos {
		fmt.Println("world at", int(at))
	}
	fmt.Println(string(s.substr(0, 5)) + "?")
	std.reverse(s.begin(), s.end())
	fmt.Println(s)
	gs := "from gocpp"
	cs := std.string(gs)
	fmt.Println(int(cs.length()), cs == "from gocpp")

	// std::filesystem through the renamed namespace fs; only paths, so the
	// output is the same on every machine.
	p := fs.path("/home/gopher/notes/todo.txt")
	fmt.Println(string(p.filename().string()), string(p.stem().string()), string(p.extension().string()))
	fmt.Println(string(p.parent_path().generic_string()))
	q := p.parent_path() / "archive" / "2026.txt"
	fmt.Println(string(q.generic_string()))
	p.replace_extension(".md")
	fmt.Println(string(p.filename().string()))
	fmt.Println(string(fs.path("a/./b/../c.txt").lexically_normal().generic_string()))
}
