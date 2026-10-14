// phase: 03

// Arrays larger than 128 KiB: a local variable is boxed and built in its box (also from a
// literal or a conversion, a shadowing one and one of several in a declaration), &[N]T{...} is
// built in its block, a parameter is passed by reference and copied into a box at the
// function's entry (still a copy, read after the call's other arguments, as in Go), and a range
// copies the array to the heap. None of them is on the C++ stack.
package main

import "fmt"

const n = 1 << 21 // 16 MiB of int64

var global [n]int64

func total(a [n]int64, k int64) int64 {
	a[0] += k // the function's own copy
	t := int64(0)
	for _, v := range a {
		t += v
	}
	return t
}

func fresh(k int64) *[n]int64 {
	return &[n]int64{n - 1: k}
}

func later() int64 {
	global[1] = 100
	return 1
}

func main() {
	c := [n]int64{5: 3}
	d := [n]int64{1, 2, 3}
	s := make([]int64, n)
	s[7] = 7
	e := [n]int64(s)
	s[7] = 70
	fmt.Println(c[5], d[2], e[7], len(e))
	global[3] = 4
	fmt.Println(total(global, later()), global[0], global[1])
	fmt.Println(total(c, 1), c[0], total(d, 0))
	sum := int64(0)
	for i, v := range d {
		if i == 0 {
			d[1] = 200 // not seen: the range copies d
		}
		sum += v
	}
	fmt.Println(sum, d[1])

	p := &[n]int64{1: 2}
	q := &[n]int64{}
	r := fresh(9)
	p[2] = q[1] + r[n-1]
	fmt.Println(p[1], p[2], len(q))
	a, b := [n]int64{1: 2}, 3
	{
		c := [n]int64{4: c[5] + a[1]} // reads the outer c
		fmt.Println(c[4], b)
	}
	fmt.Println(c[4])
}


// Output:
// 3 3 7 2097152
// 105 0 100
// 4 0 6
// 6 200
// 2 9 2097152
// 5 3
// 0
