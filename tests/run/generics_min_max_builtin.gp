// phase: 07

// The min and max built-ins on type parameters: integers, floats (NaN rules) and strings.
package main

import "fmt"

type Ordered interface {
	~int | ~int8 | ~uint8 | ~int64 | ~float32 | ~float64 | ~string
}

type Level uint8

func Lo[T Ordered](a, b, c T) T { return min(a, b, c) }
func Hi[T Ordered](a, b, c T) T { return max(a, b, c) }

func Clamp[T Ordered](x, lo, hi T) T { return min(max(x, lo), hi) }

func Extent[T Ordered](xs ...T) (T, T) {
	lo, hi := xs[0], xs[0]
	for _, x := range xs {
		lo, hi = min(lo, x), max(hi, x)
	}
	return lo, hi
}

func main() {
	fmt.Println(Lo(3, 1, 2), Hi(3, 1, 2), Lo("b", "c", "a"), Hi("b", "c", "a"))
	fmt.Println(Lo(2.5, 1.5, 3.5), Hi(2.5, 1.5, 3.5), Lo[int8](-128, 5, 127), Hi[uint8](0, 255, 7))
	fmt.Println(Lo(Level(9), Level(3), Level(5)), Hi(Level(9), Level(3), Level(5)), Hi[float32](1, 2, 3))
	zero := 0.0
	nan := zero / zero
	fmt.Println(Lo(1.0, nan, 2.0) != Lo(1.0, nan, 2.0), Hi(1.0, 2.0, nan) != Hi(1.0, 2.0, nan))
	fmt.Println(Clamp(15, 0, 10), Clamp(-5, 0, 10), Clamp(5, 0, 10), Clamp("m", "a", "f"), Clamp(0.5, 0, 1))
	fmt.Println(Extent(3, 1, 4, 1, 5))
	fmt.Println(Extent("pear", "apple", "zebra"))
	fmt.Println(Extent(Level(4)))
}

// Output:
// 1 3 a c
// 1.5 3.5 -128 255
// 3 9 3
// true true
// 10 0 5 f 0.5
// 1 5
// apple zebra
// 4 4
