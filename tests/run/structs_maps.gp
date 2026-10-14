// phase: 03

// Structs as map keys and values: keys compare field by field (a blank field doesn't count,
// NaN never equals itself, -0 equals +0), a map literal stores its constant entries first, as
// gc does (the constant key with 0 first, then the one with -0 replaces it), a value read from
// a map is a copy, and a struct of maps shares them.
package main

import "fmt"

type Point struct{ X, Y int }

type Key struct {
	name string
	_    int
	f    float64
}

type Stats struct {
	count int
	tags  []string
	seen  map[string]bool
}

func main() {
	grid := map[Point]string{{0, 0}: "origin", {1, 2}: "a"}
	grid[Point{3, 4}] = "b"
	p := Point{1, 2}
	fmt.Println(len(grid), grid[p], grid[Point{}], grid[Point{9, 9}] == "")
	delete(grid, Point{0, 0})
	v, ok := grid[Point{0, 0}]
	fmt.Println(len(grid), v == "", ok)

	keys := map[Key]int{}
	keys[Key{name: "a", f: 1}] = 1
	keys[Key{name: "a", f: 1}]++
	zero := 0.0
	keys[Key{name: "n", f: zero / zero}] = 3
	keys[Key{name: "n", f: zero / zero}] = 4
	keys[Key{name: "z", f: -zero}] = 5
	fmt.Println(len(keys), keys[Key{name: "a", f: 1}], keys[Key{name: "z", f: 0}], keys[Key{name: "n", f: zero / zero}])

	negz := -zero
	order := map[Key]int{{name: "z", f: negz}: 1, {name: "z", f: 0}: 2}
	for k, v := range order {
		fmt.Println(k.name, 1/k.f, v)
	}

	st := map[string]Stats{"x": {count: 1, seen: map[string]bool{}}}
	s := st["x"] // a copy
	s.count = 10
	s.tags = append(s.tags, "t")
	s.seen["k"] = true // the map is shared
	fmt.Println(st["x"].count, len(st["x"].tags), st["x"].seen["k"], s.count)
	st["x"] = s
	fmt.Println(st["x"].count, st["x"].tags[0])

	ptrs := map[string]*Point{"p": {5, 6}}
	ptrs["p"].X++
	fmt.Println(*ptrs["p"])
}

// Output:
// 3 a origin true
// 2 true false
// 4 2 5 0
// z -Inf 1
// 1 0 true 10
// 10 t
// {6 6}
