// phase: 08

// Escape analysis (step 15.2, ADR-114) and generics: each instance of a generic function has
// variable objects of its own, so a generic function is lowered without local closures and stack
// objects (its closures capture a box, its addresses are boxed), whatever the instances are. The
// non-generic functions beside it keep both. The output is what real Go prints, and the run is
// ASan-clean.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Add(d int) { c.n += d }

func bump(c *Counter, d int) { c.n += d * 2 }

type Number interface{ ~int | ~float64 }

// A generic function with a local closure that changes a variable, and an address-taken local.
func Sum[T Number](xs []T) (T, int) {
	var total T
	add := func(x T) { total += x }
	var c Counter
	for _, x := range xs {
		add(x)
		c.Add(1)
		bump(&c, 1)
	}
	func() { total += 1 }()
	return total, c.n
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(x T) { s.items = append(s.items, x) }

// A method of a generic type, with a closure that escapes and one that does not.
func (s *Stack[T]) Each(f func(T)) {
	n := 0
	count := func() { n++ }
	for _, x := range s.items {
		f(x)
		count()
	}
	local := Counter{}
	local.Add(n)
	fmt.Println("each", n, local.n)
}

func Make[T Number](step T) func() T {
	var v T
	return func() T { v += step; return v }
}

// The same shapes without generics.
func SumInt(xs []int) (int, int) {
	total := 0
	add := func(x int) { total += x }
	var c Counter
	for _, x := range xs {
		add(x)
		c.Add(1)
		bump(&c, 1)
	}
	func() { total += 1 }()
	return total, c.n
}

func main() {
	fmt.Println(Sum([]int{1, 2, 3}))
	fmt.Println(Sum([]float64{1.5, 2.5}))
	fmt.Println(SumInt([]int{1, 2, 3}))
	s := &Stack[string]{}
	s.Push("a")
	s.Push("b")
	total := ""
	s.Each(func(x string) { total += x })
	fmt.Println(total)
	t := &Stack[int]{}
	t.Push(5)
	sum := 0
	t.Each(func(x int) { sum += x })
	fmt.Println(sum)
	next := Make(3)
	next()
	fmt.Println(next(), Make(1.5)())
}

// Output:
// 7 9
// 5 6
// 7 9
// each 2 2
// ab
// each 1 1
// 5
// 6 1.5
