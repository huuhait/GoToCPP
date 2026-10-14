// phase: 03

// Map literals in gc's order: the entries whose key and value are constants are stored first,
// then the others in source order, so of two equal keys the later one that isn't constant wins
// (and its key is the one kept: -0 or +0); each entry is evaluated completely before the next
// one (a panic or a call in an entry comes before the calls of the next ones), and nested
// literals can leave out their types.
package main

import "fmt"

var gx = 1

func f(tag string, v int) int {
	fmt.Print(tag, " ")
	gx += 10
	return v
}

func main() {
	zero := 0.0
	negz := -zero
	m := map[float64]string{negz: "neg", 0: "pos"}
	for k, v := range m {
		fmt.Println(len(m), 1/k, v)
	}
	m2 := map[float64]string{0: "pos", negz: "neg"}
	for k, v := range m2 {
		fmt.Println(len(m2), 1/k, v)
	}

	one := 1
	dup := map[int]string{one: "var", 1: "const"}
	fmt.Println(len(dup), dup[1])
	dup2 := map[int]string{1: "const", one: "var", one * 1: "var2"}
	fmt.Println(len(dup2), dup2[1])

	gx = 1
	order := map[int]int{f("k1", 1): gx, f("k2", 2): f("v2", gx), 3: gx}
	fmt.Println(order[1], order[2], order[3])
	gx = 1
	order2 := map[string]int{"a": gx, "b": f("b", 0), "c": gx}
	fmt.Println(order2["a"], order2["b"], order2["c"])
	gx = 1
	sl := []int{gx, f("s", 0), gx}
	fmt.Println(sl[0], sl[1], sl[2])

	nested := map[string]map[string][]int{
		"a": {"x": {1, 2}, "y": nil},
		"b": {},
	}
	fmt.Println(len(nested), len(nested["a"]), len(nested["a"]["x"]), nested["a"]["y"] == nil, len(nested["b"]), nested["c"] == nil)
	ptrs := map[int]*int{1: &one, 2: nil}
	*ptrs[1] = 5
	fmt.Println(one, ptrs[2] == nil, ptrs[3] == nil)
	fmt.Println(len(map[bool]bool{}), len(map[string]int{"": 0}))
}
