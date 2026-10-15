// phase: 03

// &[]T{...} and &map[K]V{...} (step 3.12): Go puts the literal in a new variable and takes its
// address; the pointer shares nothing with other literals, also in a loop.
package main

import "fmt"

type NS []int
type M map[string]int

func f(v int) int {
	fmt.Println("f", v)
	return v
}

func main() {
	p := &[]int{f(1), f(2)}
	q := &map[string]int{"a": f(3), "b": 4}
	r := &NS{5}
	m := &M{"x": f(6)}
	ss := &[][]int{{1}, {2, 3}}
	(*ss)[1][0] = 9
	*p = append(*p, 7)
	(*q)["c"] = 8
	*r = append(*r, len(*p))
	(*m)["y"] = 1
	fmt.Println(p, *p, len(*q), (*q)["a"], *r, r, len(*m), *ss, m)
	pp := p
	*pp = nil
	fmt.Println(p, *p == nil, &[]string{}, &map[int]bool{})
	var keep *[]int
	for i := range 3 {
		s := &[]int{i}
		if i == 1 {
			keep = s
		}
	}
	fmt.Println(*keep)
}
