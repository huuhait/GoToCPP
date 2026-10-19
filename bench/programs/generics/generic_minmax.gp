// bench: generic_minmax
// category: generics
// size: 100000000
// smoke: 100000
// twin: yes
// about: generic Max, Min and Abs with a union constraint (~int | ~float64) in a hot loop
package main

import "fmt"

type Number interface {
	~int | ~int64 | ~float64
}

func Max[T Number](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Min[T Number](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Abs[T Number](a T) T {
	if a < 0 {
		return -a
	}
	return a
}

func main() {
	hi, lo := 0, 0
	fhi, flo := 0.0, 0.0
	seed := uint64(1)
	for i := 0; i < benchN; i++ {
		seed = seed*6364136223846793005 + 1442695040888963407
		v := int(seed>>40) - 1<<23
		hi = Max(hi, Abs(v))
		lo = Min(lo, v)
		f := float64(v) * 0.001
		fhi = Max(fhi, Abs(f))
		flo = Min(flo, f)
	}
	fmt.Printf("%d %d %.3f %.3f\n", hi, lo, fhi, flo)
}
