// phase: 05

// A wrapper that embeds an interface and overrides one method (the shape of sort.Reverse): the other
// methods are promoted from the embedded interface, the wrapper converts to the same interface, and
// wrappers nest.
package main

import "fmt"

type Sorter interface {
	Len() int
	Less(i, j int) bool
	Swap(i, j int)
}

type ints []int

func (s ints) Len() int           { return len(s) }
func (s ints) Less(i, j int) bool { return s[i] < s[j] }
func (s ints) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type reverse struct{ Sorter }

func (r reverse) Less(i, j int) bool { return r.Sorter.Less(j, i) }

func Reverse(s Sorter) Sorter { return &reverse{s} }

type counting struct {
	Sorter
	less, swaps *int
}

func (c counting) Less(i, j int) bool { *c.less++; return c.Sorter.Less(i, j) }
func (c counting) Swap(i, j int)      { *c.swaps++; c.Sorter.Swap(i, j) }

func sort(s Sorter) {
	for i := 1; i < s.Len(); i++ {
		for j := i; j > 0 && s.Less(j, j-1); j-- {
			s.Swap(j, j-1)
		}
	}
}

func main() {
	xs := ints{3, 1, 4, 1, 5, 9, 2, 6}
	sort(xs)
	fmt.Println(xs)
	sort(Reverse(xs))
	fmt.Println(xs)
	var less, swaps int
	sort(counting{Reverse(Reverse(xs)), &less, &swaps})
	fmt.Println(xs, less, swaps)
	var s Sorter = reverse{xs}
	_, isReverse := s.(reverse)
	_, isPtr := s.(*reverse)
	fmt.Println(isReverse, isPtr, s.Len())
}

// Output:
// [1 1 2 3 4 5 6 9]
// [9 6 5 4 3 2 1 1]
// [1 1 2 3 4 5 6 9] 28 27
// true false 8
