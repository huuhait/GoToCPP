// phase: 03

// Maps that hold other values: maps of slices (m[k] = append(m[k], x), the classic grouping),
// maps of maps (a missing inner map reads as nil), maps of pointers, a pointer into the array of
// a slice in a map, and literals with elided inner types.
package main

import "fmt"

func main() {
	words := []string{"go", "is", "fun", "and", "maps", "are", "too", "ok"}
	byLen := map[int][]string{}
	for _, w := range words {
		byLen[len(w)] = append(byLen[len(w)], w)
	}
	for n := range 5 {
		fmt.Print(n, ":")
		for _, w := range byLen[n] {
			fmt.Print(" ", w)
		}
		fmt.Println()
	}
	idx := map[string][]int{}
	for i, w := range words {
		k := w[:1]
		idx[k] = append(idx[k], i)
	}
	fmt.Println(len(idx), len(idx["a"]), idx["a"][0], idx["a"][1], idx["t"][0], idx["x"] == nil)
	idx["a"] = append(idx["a"], idx["a"]...)
	idx["a"][0] = -1
	fmt.Println(len(idx["a"]), idx["a"][0], idx["a"][2])
	p := &idx["a"][1]
	*p = 42
	fmt.Println(idx["a"][1])

	grid := map[int]map[int]string{0: {0: "a", 1: "b"}, 1: {}}
	grid[1][5] = "c"
	grid[0][1] += "!"
	fmt.Println(grid[0][0], grid[0][1], grid[1][5], len(grid[1]), grid[2] == nil, len(grid[2]), grid[2][0] == "")
	if grid[3] == nil {
		grid[3] = map[int]string{}
	}
	grid[3][3] = "d"
	fmt.Println(len(grid), grid[3][3])

	counts := map[string]map[string]int{}
	for _, w := range words {
		first := w[:1]
		if counts[first] == nil {
			counts[first] = make(map[string]int)
		}
		counts[first][w]++
	}
	fmt.Println(len(counts), len(counts["a"]), counts["a"]["are"], counts["t"]["too"], counts["z"]["z"])

	a, b := 1, 2
	ptrs := map[string]*int{"a": &a, "b": &b}
	*ptrs["a"] += 10
	ptrs["c"] = ptrs["b"]
	*ptrs["c"] = 20
	fmt.Println(a, b, ptrs["none"] == nil, len(ptrs))

	lit := map[string][]int{"x": {1, 2}, "y": nil, "z": {}}
	fmt.Println(len(lit["x"]), lit["y"] == nil, lit["z"] == nil, len(lit["z"]))
	deep := map[string]map[string][]int{"a": {"b": {7, 8}}}
	deep["a"]["b"] = append(deep["a"]["b"], 9)
	fmt.Println(len(deep["a"]["b"]), deep["a"]["b"][2])
}

// Output:
// 0:
// 1:
// 2: go is ok
// 3: fun and are too
// 4: maps
// 7 2 3 5 6 true
// 4 -1 3
// 42
// a b! c 1 true 0 true
// 3 d
// 7 2 1 1 0
// 11 20 true 3
// 2 true false 0
// 3 9
