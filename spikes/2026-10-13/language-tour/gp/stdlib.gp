package main

import (
	"fmt"

	"<algorithm>"
	"<array>"
	"<map>"
	"<string>"
	"<vector>"
)

func main() {
	nums := std.vector[int]{5, 2, 9}
	nums.push_back(1)
	std.sort(nums.begin(), nums.end())
	for i, n := range nums {
		fmt.Println(i, n)
	}

	ages := std.MAP[std.string, int]{} // "map" is a Go keyword, but it is allowed after "."
	ages["ann"] = 30

	var buf std.array[byte, N64] // a C++ template that takes a number
	fmt.Println(ages.size(), buf.size())
}
