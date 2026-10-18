// phase: 05

// Struct keys with blank fields and interface fields (step 5.6): the blank fields are neither
// compared nor hashed, an interface field takes values of several dynamic types, and the struct
// may sit in an interface (map[any]) or be a key itself.
package main

import "fmt"

type Key struct {
	id   int
	_    string
	tag  any
	_, _ float64
}

type Edge struct {
	from, to any
}

func main() {
	m := map[Key]int{}
	m[Key{id: 1, tag: "a"}] = 1
	m[Key{id: 1, tag: "a"}] += 10
	m[Key{id: 1, tag: 2}] = 3
	m[Key{id: 2, tag: nil}] = 4
	m[Key{id: 2}] += 1
	fmt.Println(len(m), m[Key{id: 1, tag: "a"}], m[Key{id: 1, tag: 2}], m[Key{id: 2}], m[Key{id: 3}])
	var k1, k2 Key
	k1.tag, k2.tag = 5, 5
	fmt.Println(k1 == k2, m[k1], len(m))

	a := map[any]string{}
	a[Key{id: 1}] = "key"
	a[Edge{1, 2}] = "edge"
	a[Edge{"x", Key{id: 1}}] = "nested"
	a[Edge{1, 2}] += "+"
	a[Edge{2, 1}] = "reverse"
	fmt.Println(len(a), a[Key{id: 1}], a[Edge{1, 2}], a[Edge{"x", Key{id: 1}}], a[Edge{2, 1}], a[Edge{}] == "")

	// A graph: edges as keys, with node values of different types.
	g := map[Edge]bool{}
	nodes := []any{1, "b", 3.5, [1]int{4}, Key{id: 5}, nil}
	for i, from := range nodes {
		for j, to := range nodes {
			if (i+j)%3 == 0 {
				g[Edge{from, to}] = true
			}
		}
	}
	count := 0
	for i, from := range nodes {
		for j, to := range nodes {
			if g[Edge{from, to}] != ((i+j)%3 == 0) {
				fmt.Println("mismatch", i, j)
			}
			if g[Edge{from, to}] {
				count++
			}
		}
	}
	fmt.Println(len(g), count)
}

// Output:
// 3 11 3 5 0
// true 0 3
// 4 key edge+ nested reverse true
// 12 12
