// bench: generic_sum_instances
// category: generics
// size: 100000000
// smoke: 20000
// twin: yes
// about: one generic Sum function instantiated with five numeric types over slices
package main

import "fmt"

func Sum[T int | int32 | uint8 | float32 | float64](xs []T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func main() {
	n := 1000
	a := make([]int, n)
	b := make([]int32, n)
	c := make([]uint8, n)
	d := make([]float32, n)
	e := make([]float64, n)
	for i := 0; i < n; i++ {
		a[i], b[i], c[i], d[i], e[i] = i, int32(i*3), uint8(i), float32(i)*0.5, float64(i)*0.25
	}
	var ta int
	var tb int32
	var tc uint8
	var td float32
	var te float64
	for r := 0; r < benchN/n; r++ {
		a[r%n] = r
		ta += Sum(a)
		b[r%n] = int32(r)
		tb += Sum(b)
		c[r%n] = uint8(r)
		tc += Sum(c)
		d[r%n] = float32(r)
		td += Sum(d)
		e[r%n] = float64(r)
		te += Sum(e)
	}
	fmt.Printf("%d %d %d %.1f %.1f\n", ta, tb, tc, td, te)
}
