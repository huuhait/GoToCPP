// phase: 03

// Sub-slices share the array: a write through one is seen through the other, s[a:b] may
// reach past len(s) up to cap(s), s[a:b:c] limits the capacity (so an append to it copies),
// and an append in place writes over the elements after a shorter slice.
package main

import "fmt"

func show(s []int) {
	fmt.Print(" [")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Print("]")
}

func main() {
	s := []int{0, 1, 2, 3, 4, 5, 6, 7}
	a := s[2:5]
	a[0] = 20
	show(s)
	show(a)
	fmt.Println(len(a), cap(a))
	show(a[1:])
	show(a[:5])
	show(a[:0])
	fmt.Println()

	b := s[1:3]
	b = append(b, 30) // in place: overwrites s[3]
	show(s)
	show(b)
	fmt.Println()

	c := s[1:3:3]
	c = append(c, 40) // a new array: s is unchanged
	c[0] = -1
	show(s)
	show(c)
	fmt.Println(cap(s[1:3:5]), len(s[1:3:5]))

	var empty []int
	show(empty[:])
	show(empty[0:0])
	fmt.Println(empty[:] == nil, s[8:] == nil, len(s[8:]))

	lo, hi, max := 1, 2, 4
	d := s[lo:hi:max]
	d = append(d, 50, 60)
	d = append(d, 70)
	d[0] = 99
	show(s)
	show(d)
	fmt.Println()
	s = s[2:]
	s = s[:len(s)-1]
	show(s)
	fmt.Println(len(s))
}

// Output:
//  [0 1 20 3 4 5 6 7] [20 3 4]3 6
//  [3 4] [20 3 4 5 6] []
//  [0 1 20 30 4 5 6 7] [1 20 30]
//  [0 1 20 30 4 5 6 7] [-1 20 40]4 2
//  [] []true false 0
//  [0 1 50 60 4 5 6 7] [99 50 60 70]
//  [50 60 4 5 6]5
