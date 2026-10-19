// phase: 05

// Results hidden inside methods and generic-free helpers: a method whose result is named like the
// receiver's field or a parameter of a closure, a receiver named like a result of an outer
// function literal, and a parameter that an inner variable hides.
package main

import "fmt"

type T struct {
	n  int
	xs []int
}

func (t *T) find(v int) (n int, ok bool) {
	defer func() { t.n++ }()
	for n, x := range t.xs {
		if x == v {
			return n, true
		}
	}
	return -1, false
}

func (t T) val(v int) (t2 T, i int) {
	defer func() { i += 1 }()
	for t := 0; t < 3; t++ {
		if t == v {
			return T{n: t}, t * 7
		}
	}
	return t, 0
}

func param(n int) (r int) {
	defer func() { r += n }()
	for n := 0; n < 5; n++ {
		if n == 3 {
			return n * 100
		}
	}
	return -1
}

func (t *T) closure() (n int) {
	defer func() {}()
	f := func(t *T) (n int, s string) {
		defer func() { s += "." }()
		for n, s := range t.xs {
			if s == 2 {
				return n, "found"
			}
		}
		return -1, "none"
	}
	a, b := f(t)
	return a + len(b)
}

func main() {
	t := &T{xs: []int{4, 5, 6}}
	fmt.Println(t.find(6))
	fmt.Println(t.find(1))
	fmt.Println(t.n)
	fmt.Println(t.val(2))
	fmt.Println(t.val(9))
	fmt.Println(param(2))
	fmt.Println(param(0))
	fmt.Println((&T{xs: []int{1, 2}}).closure())
}

// Output:
// 2 true
// -1 false
// 2
// {2 []} 15
// {2 [4 5 6]} 1
// 302
// 300
// 7
