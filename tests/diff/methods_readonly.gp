// phase: 03

// Value receivers that a body can't change are a const reference in C++ (step 3.10): reads of
// the receiver's fields, arrays, maps and slices, ranges over them, copies of it into locals
// that change, and print; the results are Go's.
package main

import "fmt"

type Celsius float64

func (c Celsius) F() Celsius { return c*9/5 + 32 }

type IDs []int

func (ids IDs) Sum() (s int) {
	for _, v := range ids {
		s += v
	}
	return
}

type Grid [3]int

func (g Grid) Max() int {
	m := g[0]
	for _, v := range g {
		if v > m {
			m = v
		}
	}
	for i := range len(g) {
		m += g[i] * 0
	}
	return m
}

type R struct {
	arr [3]int
	m   map[string]int
	s   []string
	p   *R
	n   string
}

func (r R) Info() (int, int, string, bool) {
	t := 0
	for i, v := range r.arr {
		t += i * v
	}
	for k, v := range r.m {
		t += len(k) * v
	}
	x := r
	x.n = "changed"
	println("in Info", len(r.s))
	return t, len(r.s) + cap(r.s[:1]), r.n + x.n, r.p == nil
}

func (r R) Same() R { return r }

func main() {
	fmt.Println(Celsius(100).F(), IDs{1, 2, 3}.Sum(), Grid{4, 9, 2}.Max())
	r := R{arr: [3]int{1, 2, 3}, m: map[string]int{"ab": 2}, s: []string{"x"}, n: "n"}
	fmt.Println(r.Info())
	fmt.Println(r.Same().n)
}
