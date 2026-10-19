package main

import (
	"fmt"

	"<vector>"
)

macro Dbg[T any](v T) T {
	fmt.Println(#text(v), v)
	return v
}

func main() {
	nums := std.vector[int]{5, 3, 8, 1}
	nums.push_back(4)
	n := #Dbg(int(nums.size()))
	if nums.size() == 5 {
		fmt.Println("five", n)
	}
}
